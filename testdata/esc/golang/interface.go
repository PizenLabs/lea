package testdata

type Service interface {
	Login(username, password string) bool
	Logout()
}

type User struct {
	ID   int
	Name string
}
