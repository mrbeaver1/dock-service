package dto

type Credentials struct {
	Login    string
	Password string
}

type RegisterResult struct {
	Login string `json:"login"`
}

type AuthResult struct {
	Token string `json:"token"`
}
