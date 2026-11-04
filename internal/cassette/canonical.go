package cassette

import "net/textproto"

func textprotoCanonicalMIMEHeaderKey(value string) string {
	return textproto.CanonicalMIMEHeaderKey(value)
}
