package domain

type Object struct {
	Bucket      string
	Key         string
	ContentType string
	Body        []byte
}
