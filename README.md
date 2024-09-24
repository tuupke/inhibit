# Introduction

Inhibit is a simple tool to prevent Gnome from suspending when there are active SSH sessions.

It works by simply starting `gnome-session-inhibit` in the background, and then killing it when all SSH sessions have ended.

# Installation

## From source

```bash
go install github.com/tuupke/inhibit@latest
```

## From binary

Download the latest binary from the [releases page](https://github.com/tuupke/inhibit/releases), 
and place it somewhere on your `$PATH`.

# Usage
The tool is invoked with the `inhibit` command and takes two arguments:
1. Either `login` or `logout`.
2. The pid of the SSH session.

By adding the following to your `~/.zshrc` SSH sessions will inhibit suspend:

```bash
if [[ ! -z $SSH_TTY ]]; then
    # Retrieve the current pid
    current=$$

    # Register the current pid with the inhibit tool
    inhibit login $current

    function shellExit {
        # Unregister the current pid with the inhibit tool
        inhibit logout $current
    }

    trap shellExit EXIT
fi
```
