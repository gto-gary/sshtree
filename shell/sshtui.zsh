# sshtui shell integration for zsh.
#
# After a session ends - whether you quit normally, it timed out, or the
# connection dropped - pressing the up arrow recalls the real command
# (ssh, sftp, or scp for a file copy) so you can reconnect or retry
# directly, without going back through the sshtui picker.
#
# Usage: add this to your ~/.zshrc:
#   source /path/to/sshtui/shell/sshtui.zsh
#
# How this works: `command sshtui --print-only` runs the picker and prints
# what was chosen as plain lines (never runs ssh/sftp/scp itself in this
# mode). This function captures that via $(...), then runs the real
# command itself *after* that capture has completed - so it gets a normal
# terminal, not one whose stdout is being captured (which would otherwise
# swallow scp's progress bar, or an interactive sftp prompt).
#
# This defines a shell function named `sshtui` that shadows the installed
# `sshtui` command for interactive use. Use `command sshtui` to bypass it.
sshtui() {
    local output
    output=$(command sshtui --print-only) || return
    [[ -z "$output" ]] && return

    local -a lines
    lines=("${(f)output}")

    case "${lines[1]}" in
        connect)
            local host="${lines[2]}"
            print -s -- "ssh $host"
            ssh "$host"
            ;;
        sftp)
            local host="${lines[2]}"
            print -s -- "sftp $host"
            sftp "$host"
            ;;
        scp)
            local direction="${lines[2]}" host="${lines[3]}" \
                  local_path="${lines[4]}" remote_path="${lines[5]}"
            if [[ "$direction" == "upload" ]]; then
                print -s -- "scp $local_path $host:$remote_path"
                scp "$local_path" "$host:$remote_path"
            else
                print -s -- "scp $host:$remote_path $local_path"
                scp "$host:$remote_path" "$local_path"
            fi
            ;;
    esac
}
