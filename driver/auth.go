package driver

// Auth holds credentials for SignIn and SignUp. It has the same fields and
// wire names as surrealdb.go's Auth, so the same value works with both
// backends.
//
// The access level is chosen from the fields that are set:
//   - Access set: record access (Namespace and Database are required)
//   - Namespace and Database set: database user
//   - Namespace set: namespace user
//   - none of them set: root user
//
// SignIn and SignUp also accept a map[string]any using the same keys
// ("NS", "DB", "AC", "user", "pass"). For record access, every key other
// than NS, DB and AC is passed to the access method's SIGNIN or SIGNUP
// clause as a variable, so a map can carry custom fields such as "email".
type Auth struct {
	Namespace string `json:"NS,omitempty"`
	Database  string `json:"DB,omitempty"`
	Access    string `json:"AC,omitempty"`
	Username  string `json:"user,omitempty"`
	Password  string `json:"pass,omitempty"`
}
