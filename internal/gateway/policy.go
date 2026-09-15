package gateway

import (
	"be-modami-no-service/pkg/centrifugo"
)

// ChannelPolicy decides who may subscribe to a channel in the "noti" namespace.
// It is the single access-control point for the namespace — both the subscribe
// and publish callbacks route through Allow.
type ChannelPolicy struct {
	hmacSecret string
}

func NewChannelPolicy(hmacSecret string) *ChannelPolicy {
	return &ChannelPolicy{hmacSecret: hmacSecret}
}

// Allow reports whether userID may act on channel. subToken is the subscription
// token Centrifugo forwards from the client; it is only consulted for channel
// kinds that require one.
//
// Rules:
//
//	noti:user:{id}       owner only
//	noti:topic:{id}      any authenticated user — public broadcast
//	noti:challenge:{id}  requires a subscription token minted for exactly this
//	                     user and channel by the service that knows membership
//	anything else        denied
func (p *ChannelPolicy) Allow(userID, channel, subToken string) bool {
	if userID == "" {
		return false
	}

	kind, id, ok := centrifugo.ParseNotiChannel(channel)
	if !ok {
		return false
	}

	switch kind {
	case centrifugo.KindUser:
		return id == userID

	case centrifugo.KindTopic:
		return true

	case centrifugo.KindChallenge:
		if subToken == "" {
			return false
		}
		sub, tokenChannel, err := centrifugo.ParseSubscriptionToken(p.hmacSecret, subToken)
		if err != nil {
			return false
		}
		// Both must match: a token is only good for the user and channel it names.
		return sub == userID && tokenChannel == channel

	default:
		return false
	}
}
