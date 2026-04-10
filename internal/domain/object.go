package domain

type Object struct {
	Bucket      string
	Key         string
	ContentType string
	Size        int64
	Body        []byte
}
