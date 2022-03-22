require 'socket'
require 'securerandom'
require 'hrr_rb_ssh'
require_relative './helper'

logger = Helper.get_logger
$mutex = Mutex.new
$sessions = {}

class Server

  def render_slide(io, id, num_connections, slide, current_slide, total_slides, window_width, window_height, control = true)
    io.write "\e[2J\e[H"
    io.write "\x1b[?25l" # hide cursor

    parsed_slide = Helper.glamourify(slide, window_width)
    footer = Helper.get_footer(id, num_connections, current_slide, total_slides, window_width, control)

    height = parsed_slide.count("\n") + 1
    parsed_slide = Helper.join_height(parsed_slide, footer, window_height)

    io.write parsed_slide
  end

  def start_service io, logger=nil

    auth = HrrRbSsh::Authentication::Authenticator.new { |context|
      true
    }

    conn_exit = HrrRbSsh::Connection::RequestHandler.new { |context|
      context.chain_proc { |chain|
        context.io[1].write "Usage: ssh -t peachtree.ml create [name-of-session] url-to-markdown-file\r\n"
        context.io[1].write "       ssh -t peachtree.ml join name-of-session\r\n"
      }
    }

    conn_pty_req = HrrRbSsh::Connection::RequestHandler.new { |context|
      context.chain_proc { |chain|
        context.vars[:terminal_width_characters] = context.terminal_width_characters
        context.vars[:terminal_height_rows] = context.terminal_height_rows
        chain.call_next
      }
    }

    conn_exec = HrrRbSsh::Connection::RequestHandler.new { |context|
      context.chain_proc { |chain|
        window_width = context.vars[:terminal_width_characters] || 80
        window_height = context.vars[:terminal_height_rows] || 24
        command = context.command.split(" ")

        if command.first == "create"
          if command.length == 2
            url = command[1]
            id = SecureRandom.hex(3)
          else
            url = command[2]
            id = command[1]
          end

          slides = Helper.get_slides(url)
          logger.error { "Created #{id} with #{url} "}

          $mutex.synchronize do
            $sessions[id] = {
              slides: slides,
              current_slide: 0,
              num_connections: 0,
              cv: ConditionVariable.new,
              complete: false,
            }

          end

          current_slide = 0
          num_connections = 0

          begin
            loop do

              $mutex.synchronize do
                current_slide = $sessions[id][:current_slide]
                num_connections = $sessions[id][:num_connections]
              end

              slide = slides[current_slide % slides.count]

              render_slide(context.io[1],
                id,
                num_connections,
                slide,
                current_slide,
                slides.count,
                window_width,
                window_height
              )

              buf = context.io[0].readpartial(1024)

              is_complete = false
              $mutex.synchronize do
                if buf.include?("\e[D") && $sessions[id][:current_slide] > 0
                  $sessions[id][:current_slide]-=1
                elsif buf.include?("\e[C") && $sessions[id][:current_slide] < slides.count - 1
                  $sessions[id][:current_slide]+=1
                elsif buf.include?(0x04.chr) # break if ^D
                  $sessions[id][:complete] = is_complete = true
                else
                  break
                end
                $sessions[id][:cv].broadcast

              end
              break if is_complete

            end
          rescue => e
            logger.error([e.backtrace[0], ": ", e.message, " (", e.class.to_s, ")\n\t", e.backtrace[1..-1].join("\n\t")].join)
          end

        elsif command.first == "join"
          id = command[1]
          if $sessions[id]
            slides = $sessions[id][:slides]

            $mutex.synchronize do
              $sessions[id][:num_connections] += 1
              num_connections = $sessions[id][:num_connections]
            end

            current_slide = nil
            is_complete = false

            loop do

              $mutex.synchronize do
                $sessions[id][:cv].wait($mutex) if $sessions[id][:current_slide] == current_slide && !current_slide.nil?
                current_slide = $sessions[id][:current_slide]
                is_complete = $sessions[id][:complete]
              end

              break if is_complete

              render_slide(context.io[1],
                id,
                num_connections,
                slides[current_slide],
                current_slide,
                slides.count,
                window_width,
                window_height,
                false
              )
            end

          else
            context.io[1].write "ID was not found\r\n"
          end

        else
          context.io[1].write "Unknown command\r\n"
        end

        context.io[1].write "\r\n"
        context.io[1].write "\x1b[?25h" # show cursor again

      }
    }

    options = {}

    options['authentication_preferred_authentication_methods'] = ["none"]
    options['authentication_none_authenticator']      = auth
    options['authentication_publickey_authenticator'] = auth
    options['authentication_password_authenticator']  = auth

    options['connection_channel_request_shell']      = conn_exit
    options['connection_channel_request_exec']       = conn_exec
    options['connection_channel_request_pty_req']    = conn_pty_req

    options['transport_server_secret_host_keys'] = {}
    options['transport_preferred_server_host_key_algorithms'] = %w(ssh-rsa ssh-dss)
    options['transport_server_secret_host_keys']['ssh-rsa'] = File.read('./id_rsa')

    server = HrrRbSsh::Server.new options, logger: logger
    server.start io
  end

end


server = TCPServer.new 10022
s = Server.new
loop do
  Thread.new(server.accept) do |io|
    begin
      s.start_service io, logger
      io.close rescue nil
    rescue => e
      logger.error { [e.backtrace[0], ": ", e.message, " (", e.class.to_s, ")\n\t", e.backtrace[1..-1].join("\n\t")].join }
    end
  end
end
