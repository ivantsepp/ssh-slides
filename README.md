# SSH Slides

SSH Slides is an SSH server that hosts terminal-based presentations where your viewers can follow along in their own terminals.

> image here


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


