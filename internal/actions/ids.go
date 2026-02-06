package actions

// Action IDs for type-safe references throughout the codebase.
const (
	// Backend actions
	ActionBackend          = "backend"
	ActionBackendList      = "backend.list"
	ActionBackendAvailable = "backend.available"
	ActionBackendAdd       = "backend.add"
	ActionBackendRemove    = "backend.remove"
	ActionBackendStatus    = "backend.status"

	// Tunnel actions
	ActionTunnel            = "tunnel"
	ActionTunnelList        = "tunnel.list"
	ActionTunnelAdd         = "tunnel.add"
	ActionTunnelRemove      = "tunnel.remove"
	ActionTunnelEnable      = "tunnel.enable"
	ActionTunnelDisable     = "tunnel.disable"
	ActionTunnelStart       = "tunnel.start"
	ActionTunnelStop        = "tunnel.stop"
	ActionTunnelRestart     = "tunnel.restart"
	ActionTunnelStatus      = "tunnel.status"
	ActionTunnelLogs = "tunnel.logs"

	// Router actions
	ActionRouter        = "router"
	ActionRouterStatus  = "router.status"
	ActionRouterStart   = "router.start"
	ActionRouterStop    = "router.stop"
	ActionRouterRestart = "router.restart"
	ActionRouterLogs    = "router.logs"
	ActionRouterMode    = "router.mode"
	ActionRouterSwitch  = "router.switch"

	// Config actions
	ActionConfig         = "config"
	ActionConfigLoad     = "config.load"
	ActionConfigExport   = "config.export"
	ActionConfigValidate = "config.validate"

	// System actions
	ActionInstall   = "install"
	ActionUninstall = "uninstall"
	ActionSSHUsers  = "ssh-users"
	ActionUpdate    = "update"
)
