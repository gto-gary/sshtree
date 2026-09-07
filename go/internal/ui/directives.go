package ui

// commonDirectives are offered as a pickable list in the edit form so a
// directive name doesn't have to be typed from memory. Anything else is
// still supported by typing directly into the row's key field — ssh_config
// has roughly 90 directives, and a host may need one outside this
// shortlist. Kept identical to host_edit.py's COMMON_DIRECTIVES.
var commonDirectives = []string{
	"AddKeysToAgent", "AddressFamily", "BatchMode", "BindAddress",
	"CanonicalizeHostname", "Ciphers", "Compression", "ConnectTimeout",
	"ControlMaster", "ControlPath", "ControlPersist", "DynamicForward",
	"EscapeChar", "ExitOnForwardFailure", "ForwardAgent", "ForwardX11",
	"GatewayPorts", "HashKnownHosts", "HostKeyAlgorithms", "IdentitiesOnly",
	"IdentityAgent", "IdentityFile", "KbdInteractiveAuthentication",
	"KexAlgorithms", "LocalCommand", "LocalForward", "LogLevel", "MACs",
	"PasswordAuthentication", "PermitLocalCommand", "PreferredAuthentications",
	"ProxyCommand", "ProxyJump", "PubkeyAcceptedKeyTypes", "PubkeyAuthentication",
	"RemoteCommand", "RemoteForward", "RequestTTY", "SendEnv",
	"ServerAliveCountMax", "ServerAliveInterval", "SetEnv", "StrictHostKeyChecking",
	"TCPKeepAlive", "Tunnel", "UserKnownHostsFile", "VisualHostKey",
	"XAuthLocation",
}
