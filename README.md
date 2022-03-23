# SSH Slides

SSH Slides is an SSH server that hosts terminal-based presentations where your viewers can follow along in their own terminals. This service is currently located at `slides.tseivan.com`.

![Screenshot of SSH Slides](./screenshot.png "Screenshot of SSH Slides")

## Usage

All you need is a markdown file containing your presentation. To create a new session:

```bash
ssh -t slides.tseivan.com create URL_TO_RAW_MARKDOWN

# To create a session with your own unique name
ssh -t slides.tseivan.com create personal-unique-name URL_TO_RAW_MARKDOWN

# Try out our example presentation
ssh -t slides.tseivan.com create https://raw.githubusercontent.com/ivantsepp/ssh-slides/master/example_presentation.md
```

You will then be entered into a new presentation session where you have control of the slides. Your viewers can then join your session by running the following in their own terminals:

```bash
ssh -t slides.tseivan.com join SESSION_ID
```

Your viewers should then see the same content that you are seeing!

### Navigation

As the creator of the session you can:

1. To go to the next slide, press any of the following keys: `space`, `right`, `down`, `enter`, `n`, `j`, `l`
2. To go to the previous slide, press any of the following keys: `left`, `up`, `p`, `h`, `k`
3. To exit and finish the presentation session, press any of the following keys: `ctrl+c`, `ctrl+d`, `esc`, `q`

As the viewer of the session you can:
1. To exit and leave, press any of the following keys: `ctrl+c`, `ctrl+d`, `esc`, `q`


### Implementation / Notes

This idea was heavily inspired by this [amazing talk on SSH](https://vimeo.com/54505525). In the presentation, the speaker used a host where viewers could SSH in and view the slides in their own terminals. This was a really cool hack/idea to me and I wanted to challenge myself by extending that idea to provide a service for anyone to host an SSH presentation session. I quickly hacked on this project over my sabbatical and I used it as a learning experience as well to understand the underlying SSH protocol.


Since this was a learning experience, I went with Ruby as it's one of my favorite languages. I might revisit this decision as I've seen some awesome Go libraries that make it easy to hack on SSH apps like [wish](https://github.com/charmbracelet/wish). There are also amazing Go libraries written by the same folks for terminal hacking. I saw how beautiful the terminal slides looked in [glamour](https://github.com/charmbracelet/glamour) and [slides](https://github.com/maaslalani/slides). Those projects looked so good that I ended up deciding to use them in my project despite them being in a different language. I followed [this gist guide](https://gist.github.com/schweigert/385cd8e2267140674b6c4818d8f0c373) to bridge the Go code to my Ruby code.

As you can tell, this project is very barebones but please bare with me as I continue to polish and improve this project!
