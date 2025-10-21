package types

type Job struct {
	URL string
}

type Result struct {
	URL   string
	Match bool
	Err   error
}
