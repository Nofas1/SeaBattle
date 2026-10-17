package api

type AuthRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

type AuthBox int

const (
	BoxNone AuthBox = iota
	BoxUsername
	BoxPassword
)