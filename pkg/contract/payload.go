package contract

// BaseEvent is the RDF-like payload: actor (who), do (main object), io (indirect), po (prepositional).
type BaseEvent struct {
	Actor   EventObject   `json:"actor"`             // who performed (user/system)
	Do      []EventObject `json:"do"`                // main object(s): post, comment, article, ...
	Io      []EventObject `json:"io,omitempty"`      // indirect: e.g. post that contains the comment
	Po      []EventObject `json:"po,omitempty"`     // prepositional: e.g. group
	Context interface{}   `json:"context,omitempty"` // extra per-identity data
}
