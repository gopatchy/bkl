package bkl

type (
	decodeFunc func([]byte) (any, error)
	encodeFunc func(any) ([]byte, error)
)

type format struct {
	decode decodeFunc
	encode encodeFunc
}

var formatByExtension = map[string]format{
	"json": {
		decode: decodeJSON,
		encode: encodeJSON,
	},
	"yaml": {
		decode: decodeYAML,
		encode: encodeYAML,
	},
}
