package status

type AddObjectOption interface {
	ApplyToAdd(*AddObjectOptions)
}

type AddObjectOptions struct {
	MetaName       string
	GroupingObject bool
	NoEcho         bool
}

func (o *AddObjectOptions) ApplyOptions(opts []AddObjectOption) *AddObjectOptions {
	for _, opt := range opts {
		opt.ApplyToAdd(o)
	}
	return o
}

// The ObjectMetaName option defines the meta name that should be used for the object in the presentation layer,
// e.g. control plane for KCP.
type ObjectMetaName string

func (n ObjectMetaName) ApplyToAdd(options *AddObjectOptions) {
	options.MetaName = string(n)
}

// The GroupingObject option defines if the child of this node will be grouped in case the ready condition
// has the same Status, Severity and Reason.
type GroupingObject bool

func (n GroupingObject) ApplyToAdd(options *AddObjectOptions) {
	options.GroupingObject = bool(n)
}

// The NoEcho options defines if the object should be hidden if the object's ready condition has the
// same Status, Severity and Reason of the parent's object ready condition (it is an echo).
type NoEcho bool

func (n NoEcho) ApplyToAdd(options *AddObjectOptions) {
	options.NoEcho = bool(n)
}
