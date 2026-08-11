# sshtui shell integration for bash.
#
# After an SSH session ends - whether you quit normally, it timed out, or
# the connection dropped - pressing the up arrow recalls "ssh <alias>" so
# you can reconnect directly, without going back through the sshtui picker.
#
# Usage: add this to your ~/.bashrc:
#   source /path/to/sshtui/shell/sshtui.bash
#
# This defines a shell function named `sshtui` that shadows the installed
# `sshtui` command for interactive use. Use `command sshtui` to bypass it.
sshtui() {
    local host
    host=$(command sshtui --print-only) || return
    [[ -z "$host" ]] && return
    history -s "ssh $host"
    ssh "$host"
}
