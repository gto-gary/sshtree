# sshtui shell integration for zsh.
#
# After an SSH session ends - whether you quit normally, it timed out, or
# the connection dropped - pressing the up arrow recalls "ssh <alias>" so
# you can reconnect directly, without going back through the sshtui picker.
#
# Usage: add this to your ~/.zshrc:
#   source /path/to/sshtui/shell/sshtui.zsh
#
# This defines a shell function named `sshtui` that shadows the installed
# `sshtui` command for interactive use. Use `command sshtui` to bypass it.
sshtui() {
    local host
    host=$(command sshtui --print-only) || return
    [[ -z "$host" ]] && return
    print -s -- "ssh $host"
    ssh "$host"
}
