package actions

import (
	"github.com/algorand/go-algorand-sdk/v2/types"
	"net/url"
)

type Metadata struct {
	name      string
	avatarURL url.URL
	address   types.Address
}

func (meta *Metadata) UpdateMeta(name string, avatarURL url.URL, address types.Address) {

}
