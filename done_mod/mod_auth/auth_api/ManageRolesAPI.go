package authapi

import (
	"github.com/hiveot/hub/done_cli/cli_done/transport"
	modbus "github.com/hiveot/hub/done_mod/mod_bus"
)

const DefaultAclFilename = "authz.acl"

// AuthManageRolesCapability is the name of the Thing/Capability that handles role requests
const AuthManageRolesCapability = "manageRoles"

// Predefined user roles.
const (

	// ClientRoleNone indicates that the user has no particular role. It can not do anything until
	// the role is upgraded to viewer or better.
	//  Read permissions: none
	//  Write permissions: none
	ClientRoleNone = ""

	// ClientRoleAdmin lets a client publish and subscribe to any sources and invoke all services
	//  Read permissions: subEvents, subActions
	//  Write permissions: pubEvents, pubActions, pubConfig
	ClientRoleAdmin = "admin"

	// ClientRoleDevice lets a client publish things events and subscribe to device actions
	//  Read permissions: subActions
	//  Write permissions: pubTDs, pubEvents
	ClientRoleDevice = "device"

	// ClientRoleManager lets a client subscribe to Thing TD, events, publish actions and update configuration
	//  Read permissions: subEvents
	//  Write permissions: pubActions, pubConfig
	ClientRoleManager = "manager"

	// ClientRoleOperator lets a client subscribe to events and publish actions
	//  Read permissions: subEvents
	//  Write permissions: pubActions
	ClientRoleOperator = "operator"

	// ClientRoleService lets a client acts as an admin user and a device
	//  Read permissions: subEvents, subActions, subConfig
	//  Write permissions: pubEvents, pubActions, pubConfig
	ClientRoleService = "service"

	// ClientRoleViewer lets a client subscribe to Thing TD and Thing Events
	//  Read permissions: subEvents
	//  Write permissions: none
	ClientRoleViewer = "viewer"
)

// Role based ACL matrix example
// -----------------------------
// role       pub/sub   stype   agentID    thingID
//
// *	      sub       _INBOX  {clientID}   -       	(built-in rule)
// *	      pub       rpc     auth         profile 	(built-in rule)
// *          pub       any     -            -        senderID must be clientID except for inbox
//
// viewer     sub       event   -            -
// operator   pub       action  -            -
//            sub       event   -            -
// manager    pub       action  -            -
//            pub       config  -            -
//            sub       event   -            -
// admin      pub       action  -            -
//            sub       event   -            -
// device     pub       event   {clientID}   -
//            sub       event   -		     -
//            sub       action  {clientID}   -
// service    pub       -       -            -
//            sub       action  {clientID}   -
//            sub       rpc     {clientID}   -
//            sub       event   -            -

// {clientID} is replaced with the client's loginID when publishing or subscribing

// devices can publish events, replies and subscribe to their own actions and config
var devicePermissions = []modbus.RolePermission{
	{
		MsgType:  transport.MessageTypeEvent,
		AgentID:  "{clientID}", // devices can only publish their own events
		AllowPub: true,
	}, {
		MsgType:  transport.MessageTypeEvent,
		AgentID:  "", // devices can subscribe to events
		AllowSub: true,
	}, {
		MsgType:  transport.MessageTypeAction,
		AgentID:  "{clientID}",
		AllowSub: true,
	}, {
		MsgType:  transport.MessageTypeConfig,
		AgentID:  "{clientID}",
		AllowSub: true,
	},
}

// viewers can subscribe to all things
var viewerPermissions = []modbus.RolePermission{{
	MsgType:  transport.MessageTypeEvent,
	AllowSub: true,
}}

// operators can subscribe to events and publish things actions
var operatorPermissions = []modbus.RolePermission{
	{
		MsgType:  transport.MessageTypeEvent,
		AllowSub: true,
	}, {
		MsgType:  transport.MessageTypeAction,
		AllowPub: true,
	},
}

// managers can in addition to operator also publish configuration
var managerPermissions = append(operatorPermissions, modbus.RolePermission{
	MsgType:  transport.MessageTypeConfig,
	AllowPub: true,
})

// administrators can in addition to operators publish all RPCs
// RPC request permissions for roles are set by the service when they register.
var adminPermissions = append(managerPermissions, modbus.RolePermission{
	MsgType:  transport.MessageTypeRPC,
	AllowPub: true,
})

// services are admins that can also publish events and subscribe to their own rpc, actions and config
var servicePermissions = append(adminPermissions, modbus.RolePermission{
	MsgType:  transport.MessageTypeEvent,
	AgentID:  "{clientID}",
	AllowPub: true,
}, modbus.RolePermission{
	MsgType:  transport.MessageTypeRPC,
	AgentID:  "{clientID}",
	AllowSub: true,
}, modbus.RolePermission{
	MsgType:  transport.MessageTypeAction,
	AgentID:  "{clientID}",
	AllowSub: true,
}, modbus.RolePermission{
	MsgType:  transport.MessageTypeConfig,
	AgentID:  "{clientID}",
	AllowSub: true,
})

// DefaultRolePermissions contains the default pub/sub permissions for each user role
var DefaultRolePermissions = map[string][]modbus.RolePermission{
	ClientRoleNone:     nil,
	ClientRoleDevice:   devicePermissions,
	ClientRoleService:  servicePermissions,
	ClientRoleViewer:   viewerPermissions,
	ClientRoleOperator: operatorPermissions,
	ClientRoleManager:  managerPermissions,
	ClientRoleAdmin:    adminPermissions,
}

// AuthRolesCapability defines the 'capability' address part used in sending messages
const AuthRolesCapability = "roles"

// CreateRoleReq defines the request to create a new custom role
const CreateRoleReq = "createRole"

type CreateRoleArgs struct {
	Role string `json:"role"`
}

// DeleteRoleReq defines the request to delete a custom role.
const DeleteRoleReq = "deleteRole"

type DeleteRoleArgs struct {
	Role string `json:"role"`
}
