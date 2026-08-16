# sshtui shell integration for bash.
#
# After a session ends - whether you quit normally, it timed out, or the
# connection dropped - pressing the up arrow recalls the real command
# (ssh, sftp, or scp for a file copy) so you can reconnect or retry
# directly, without going back through the sshtui picker.
#
# Usage: add this to your ~/.bashrc:
#   source /path/to/sshtui/shell/sshtui.bash
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
    mapfile -t lines <<< "$output"

    case "${lines[0]}" in
        connect)
            local host="${lines[1]}"
            history -s "ssh $host"
            ssh "$host"
            ;;
        record)
            local host="${lines[1]}" logfile="${lines[2]}"
            mkdir -p "$(dirname "$logfile")"
            history -s "ssh $host"
            if [[ "$(uname)" == "Darwin" ]]; then
                script -q "$logfile" ssh "$host"
            else
                script -q -c "ssh $(printf '%q' "$host")" "$logfile"
            fi
            ;;
        sftp)
            local host="${lines[1]}"
            history -s "sftp $host"
            sftp "$host"
            ;;
        scp)
            local direction="${lines[1]}" host="${lines[2]}" \
                  local_path="${lines[3]}" remote_path="${lines[4]}"
            if [[ "$direction" == "upload" ]]; then
                history -s "scp $local_path $host:$remote_path"
                scp "$local_path" "$host:$remote_path"
            else
                history -s "scp $host:$remote_path $local_path"
                scp "$host:$remote_path" "$local_path"
            fi
            ;;
    esac
}
