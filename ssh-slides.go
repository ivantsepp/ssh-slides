package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    "sync"
    "io/ioutil"
    "regexp"
    "strings"
    "crypto/rand"
    "encoding/hex"
    "strconv"
    "net/http"
    "errors"

    "github.com/charmbracelet/bubbles/viewport"
    "github.com/charmbracelet/glamour"
    "github.com/charmbracelet/lipgloss"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/wish"
    lm "github.com/charmbracelet/wish/logging"
    "github.com/gliderlabs/ssh"
)

const host = "0.0.0.0"
const serverName = "slides.tseivan.com"
var port = 23234

var db sync.Map

func addDemoSlides() {
    demoSession := NewSession("demo", GetDemoSlides())
    db.Store("demo", demoSession)

    ticker := time.NewTicker(10 * time.Second)
    go func() {
        for {
           select {
            case <- ticker.C:
                demoSession.NextSlideLoop()
            }
        }
     }()
}

func main() {
    addDemoSlides()

    portEnv := os.Getenv("PORT")
    if len(portEnv) != 0 {
        port, _ = strconv.Atoi(portEnv)
    }

    s, err := wish.NewServer(
        wish.WithAddress(fmt.Sprintf("%s:%d", host, port)),
        wish.WithHostKeyPath("id_ed25519"),
        wish.WithMiddleware(
            Middleware(),
            lm.Middleware(),
        ),
    )
    if err != nil {
        log.Fatalln(err)
    }

    done := make(chan os.Signal, 1)
    signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
    log.Printf("Starting SSH server on %s:%d", host, port)
    go func() {
        if err = s.ListenAndServe(); err != nil {
            log.Fatalln(err)
        }
    }()

    <-done
    log.Println("Stopping SSH server")
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer func() { cancel() }()
    if err := s.Shutdown(ctx); err != nil {
        log.Fatalln(err)
    }
}

func printHelp(s ssh.Session) {
    fmt.Fprintln(s, "Usage: ssh -t", serverName, "create [name-of-session] url-to-markdown-file")
    fmt.Fprintln(s, "       ssh -t", serverName, "join name-of-session")
    fmt.Fprintln(s, "       ssh -t", serverName, "join demo # join a demo session that advances the slides every 30 seconds!")
}

func Middleware() wish.Middleware {
    return func(sh ssh.Handler) ssh.Handler {
        return func(s ssh.Session) {

            isAdmin := false
            command := s.Command()
            sessionId := ""

            if len(command) > 0 {
                switch command[0] {
                    case "create":
                        var id, url string
                        if len(command) == 2 {
                          id, _ = RandomHex(3)
                          url = command[1]
                        } else if len(command) > 2 {
                          id = command[1]
                          url = command[2]
                        }
                        if id != "" && url != "" {

                            res, _ := db.Load(id)
                            if res != nil && !res.(*Session).Complete {
                                fmt.Fprintln(s, "Session with that ID already exists")
                                return
                            }

                            slides, err := GetSlides(url)
                            if err == nil {
                                session := NewSession(id, slides)
                                db.Store(id, session)
                                sessionId = id
                                isAdmin = true
                            } else {
                                fmt.Fprintln(s, err)
                            }
                        }
                    case "join":
                        if len(command) >= 2 {
                            sessionId = command[1]
                        }
                }
            }

            if sessionId == "" {
                printHelp(s)
                return
            }

            errc := make(chan error, 1)
            pty, windowChanges, _ := s.Pty()

            res, ok := db.Load(sessionId)
            if !ok {
                fmt.Fprintln(s, "ID was not found")
                return
            }

            session := res.(*Session)

            connectionChannel := make(chan struct{}, 1)
            session.ConnectionChannels = append(session.ConnectionChannels, connectionChannel)

            m := model{
                Term:   pty.Term,
                Width:  pty.Window.Width,
                Height: pty.Window.Height,
                Slides: session.Slides,
                Style: "dark",
                NumConnections: session.NumConnections,
                Session: session,
                Complete: false,
                isAdmin: isAdmin,
                Channel: connectionChannel,
            }

            if !isAdmin {
                session.IncreaseNumConnections()
            }

            opts := append([]tea.ProgramOption{tea.WithAltScreen()}, tea.WithInput(s), tea.WithOutput(s))
            p := tea.NewProgram(m, opts...)

            go func() {
                for {
                    select {
                    case <-s.Context().Done():
                        return
                    case w := <-windowChanges:
                        if p != nil {
                            p.Send(tea.WindowSizeMsg{Width: w.Width, Height: w.Height})
                        }
                    case err := <-errc:
                        if err != nil {
                            log.Print(err)
                        }
                        return
                    case <-connectionChannel:
                        session.Lock.RLock()
                        currentSlide := session.CurrentSlide
                        isComplete := session.Complete
                        numConnections := session.NumConnections
                        session.Lock.RUnlock()

                        if isComplete {
                            p.Send(tea.Quit())
                            return
                        }
                        p.Send(UpdateMsg{CurrentSlide: currentSlide, NumConnections: numConnections})
                    }
                }
            }()
            errc <- p.Start()
            p.Kill()
        }
    }
}

type model struct {
    Term   string
    Width  int
    Height int
    Slides []string
    Style string
    CurrentSlide int
    NumConnections int
    isAdmin bool
    Complete bool
    Session *Session
    Channel chan struct{}
}

func (m model) Init() tea.Cmd {
    return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.Height = msg.Height
        m.Width = msg.Width
    case tea.KeyMsg:
        if m.isAdmin {
            switch msg.String() {
            case "q", "ctrl+c", "ctrl+d", "esc":
                m.Session.Finish()
                return m, tea.Quit
            case " ", "down", "j", "right", "l", "enter", "n", "pgdown":
                m.Session.NextSlide()
            case "up", "k", "left", "h", "p", "pgup":
                m.Session.PreviousSlide()
            case "t":
                if m.Style == "light" {
                    m.Style = "dark"
                } else {
                    m.Style = "light"
                }
                return m, nil
            }

        } else {
            switch msg.String() {
            case "q", "ctrl+c", "ctrl+d", "esc":
                m.Session.DecreaseNumConnections(m.Channel)
                return m, tea.Quit
            case "t":
                if m.Style == "light" {
                    m.Style = "dark"
                } else {
                    m.Style = "light"
                }
                return m, nil
            }
        }

    case UpdateMsg:
        m.CurrentSlide = msg.CurrentSlide
        m.NumConnections = msg.NumConnections
    }
    return m, nil
}

func (m model) View() string {

    vp := viewport.New(m.Width - 2, m.Height - 3)

    vp.Style = lipgloss.NewStyle().
        BorderStyle(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("62"))

    renderer, err := glamour.NewTermRenderer(
        glamour.WithStandardStyle(m.Style),
        glamour.WithWordWrap(m.Width - 2),
    )

    if err != nil {
        return "Error"
    }

    length := len(m.Slides)

    redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
    magentaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))

    slideInfo := magentaStyle.Render("Slide ") +
        magentaStyle.Bold(true).Render(strconv.Itoa(m.CurrentSlide + 1)) +
        magentaStyle.Render(" out of ") +
        magentaStyle.Bold(true).PaddingRight(2).Render(strconv.Itoa(length))

    var connInfo string

    if m.NumConnections == 0 {
        connInfo = redStyle.PaddingLeft(2).Render("Your join key is: ") + redStyle.Bold(true).Render(m.Session.Name)
    } else {
        connInfo = redStyle.PaddingLeft(2).Render("Number of Viewers: ") + redStyle.Bold(true).Render(strconv.Itoa(m.NumConnections))
    }

    footer := JoinHorizontal(connInfo, slideInfo, m.Width)
    content := m.Slides[m.CurrentSlide]

    str, err := renderer.Render(content)

    if err != nil {
        return "Error"
    }

    vp.SetContent(str)

    return vp.View() + "\n" + footer
}

func JoinHorizontal(left, right string, width int) string {
    length := lipgloss.Width(left + right)
    if width < length {
        return left + " " + right
    }
    padding := strings.Repeat(" ", width - length)
    return left + padding + right
}


func GetSlides(url string) ([]string, error) {
    resp, err := http.Get(url)

    if err != nil || resp.StatusCode != 200 {
        return nil, errors.New("Unable to fetch url")
    }

    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)

    if err != nil {
        return nil, errors.New("Unable to read body")
    }

    slides := string(body)
    slides = RemoveFrontmatter(slides)
    return strings.Split(slides, "\n---\n\n"), nil
}

func GetDemoSlides() ([]string) {
    buf, _ := ioutil.ReadFile("example_presentation.md")
    slides := string(buf)
    slides = RemoveFrontmatter(slides)
    return strings.Split(slides, "\n---\n\n")
}

func RemoveFrontmatter(text string) string {
    if text[0:3] == "---" {
        frontmatterRegex := regexp.MustCompile(`(?m)^(---\s*\n.*?\n?)^(---\s*$\n?)`)
        return frontmatterRegex.ReplaceAllString(text, "")
    } else {
        return text
    }
}

// https://sosedoff.com/2014/12/15/generate-random-hex-string-in-go.html
func RandomHex(n int) (string, error) {
    bytes := make([]byte, n)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return hex.EncodeToString(bytes), nil
}


type Session struct {
    Lock *sync.RWMutex
    Name string
    Slides []string
    CurrentSlide int
    NumConnections int
    Complete bool
    ConnectionChannels []chan struct{}
}

func (s *Session) NextSlide() {
    if s.CurrentSlide < len(s.Slides) - 1 {
        s.Lock.Lock()
        s.CurrentSlide += 1
        s.Lock.Unlock()
        s.broadcast()
    }
}

func (s *Session) NextSlideLoop() {
    s.Lock.Lock()
    s.CurrentSlide = (s.CurrentSlide + 1) % len(s.Slides)
    s.Lock.Unlock()
    s.broadcast()
}

func (s *Session) PreviousSlide() {
    if s.CurrentSlide >= 1 {
        s.Lock.Lock()
        s.CurrentSlide -= 1
        s.Lock.Unlock()
        s.broadcast()
    }
}

func (s *Session) Finish() {
    s.Lock.Lock()
    s.Complete = true
    s.Lock.Unlock()
    s.broadcast()
}

func (s *Session) DecreaseNumConnections(originalChannel chan struct{}) {
    s.Lock.Lock()
    s.NumConnections -= 1
    index := 0
    for i, c := range s.ConnectionChannels {
        if c == originalChannel {
            index = i
        }
    }
    log.Print("removed this index")
    log.Print(index)
    s.ConnectionChannels = append(s.ConnectionChannels[:index], s.ConnectionChannels[index+1:]...)
    s.Lock.Unlock()
    s.broadcast()
}

func (s *Session) IncreaseNumConnections() {
    s.Lock.Lock()
    s.NumConnections += 1
    s.Lock.Unlock()
    s.broadcast()
}

func (s *Session) broadcast() {
    for _, c := range s.ConnectionChannels {
        c <- struct{}{}
    }
}

type UpdateMsg struct{
    CurrentSlide int
    NumConnections int
}

func NewSession(name string, slides []string) *Session {
    s := &Session{
        Name: name,
        Slides: slides,
        CurrentSlide: 0,
        NumConnections: 0,
        Complete: false,
    }
    s.Lock = &sync.RWMutex{}
    return s
}
