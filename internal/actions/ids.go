package actions

// Action IDs for type-safe references throughout the codebase.
const (
	// Instance actions
	ActionInstance            = "instance"
	ActionInstanceList        = "instance.list"
	ActionInstanceAdd         = "instance.add"
	ActionInstanceRemove      = "instance.remove"
	ActionInstanceStart       = "instance.start"
	ActionInstanceStop        = "instance.stop"
	ActionInstanceStatus      = "instance.status"
	ActionInstanceLogs        = "instance.logs"
	ActionInstanceReconfigure = "instance.reconfigure"

	// Router actions
	ActionRouter       = "router"
	ActionRouterStatus = "router.status"
	ActionRouterStart  = "router.start"
	ActionRouterStop   = "router.stop"
	ActionRouterLogs   = "router.logs"
	ActionRouterMode   = "router.mode"
	ActionRouterSwitch = "router.switch"

	// System actions
	ActionInstall   = "install"
	ActionUninstall = "uninstall"
	ActionSSHUsers  = "ssh-users"
)
