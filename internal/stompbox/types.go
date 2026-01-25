package stompbox

type FileOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type FileTreeDef struct {
	Plugin   string       `json:"plugin"`
	Param    string       `json:"param"`
	Category string       `json:"category,omitempty"`
	Items    []string     `json:"items,omitempty"`
	Options  []FileOption `json:"options,omitempty"`
}
