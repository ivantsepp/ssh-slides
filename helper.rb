require 'ffi'
require 'net/http'
require 'logger'
require 'rainbow'
require 'strings-ansi'

Rainbow.enabled = true

module Helper

  TEST_MARKDOWN = File.read('./example_presentation.md')

  def self.get_footer(id, num_connections, current_slide, total_slides, max_width, control=true)
    if control && num_connections == 0
        message = "  Your join key is: #{Rainbow(id).red.bright}"
    else
      message = "  Number of Viewers: #{Rainbow(num_connections).red.bright}"
    end
    slides = self.get_slide_text(current_slide, total_slides)
    self.join_width(message, slides, max_width)
  end

  def self.get_slide_text(current_slide, total_slides)
    Rainbow("Slide ").magenta +
      Rainbow("#{(current_slide % total_slides) + 1}").magenta.bright +
      Rainbow(" out of ").magenta +
      Rainbow("#{total_slides}  ").magenta.bright
  end

  def self.get_slides(url)
    markdown = Net::HTTP.get(URI(url)) rescue nil
    slides = markdown || self::TEST_MARKDOWN
    slides = self.remove_frontmatter(slides)
    slides = slides.split("\n---\n\n")
  end

  # https://practicingruby.com/articles/tricks-for-working-with-text-and-files
  def self.remove_frontmatter(text)
    if text[0..2] == "---"
      return text.sub(/^(---\s*\n.*?\n?)^(---\s*$\n?)/m, "")
    else
      return text
    end
  end

  def self.join_width(left, right, max_width)
    total_width = Strings::ANSI.sanitize(left).length + Strings::ANSI.sanitize(right).length
    if max_width < total_width
      return left + " " + right
    end
    left + (" " * (max_width - total_width - 1)) + right
  end

  def self.join_height(body, footer, max_height)
    height = body.count("\n") + 1
    glue = height > max_height  ? "\r\n" : "\r\n" * (max_height - height)
    body + glue + footer
  end

  def self.glamourify(body, width)
    gostr = GoBinding::GoString.new
    gostr[:p] = FFI::MemoryPointer.from_string(body)
    gostr[:len] = body.size
    GoBinding.Glamourify(gostr, width).gsub(/\n/, "\r\n")
  end

  def self.get_logger
    STDOUT.sync = true
    logger = Logger.new STDOUT
    logger.level = Logger::ERROR
    logger.formatter = LoggerFormatter.new
    logger
  end

  def self.is_left_key(string)
    string.include?("\e[D") || string.include?("\e[A") || string.include?("p") || string.include?("h") || string.include?("k")
  end

  def self.is_right_key(string)
    string.include?(" ") || string.include?("\e[C") || string.include?("\e[B") || string.include?("\n") || string.include?("n") || string.include?("j") || string.include?("l")
  end

  def self.is_exit_key(string)
    string.include?(0x04.chr) || string.include?("\u0003") || string.include?("\e\e") || string.include?("q")
  end
end


class LoggerFormatter < ::Logger::Formatter
  def call severity, time, progname, msg
    "%s, [%s#%d.%x] %5s -- %s: %s\n" % [severity[0..0], format_datetime(time), Process.pid, Thread.current.object_id, severity, progname, msg2str(msg)]
  end
end

module GoBinding

  extend FFI::Library

  ffi_lib './my_lib.so'

  class GoString < FFI::Struct
    layout :p,     :pointer,
           :len,   :long_long
  end

  attach_function :Glamourify, [GoString.by_value, :int], :string
end
