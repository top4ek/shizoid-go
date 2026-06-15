package handlers

import (
	"testing"

	tgmodels "github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestIsJoinTransition(t *testing.T) {
	user := tgmodels.User{ID: 1, FirstName: "a"}

	tests := []struct {
		name string
		old  tgmodels.ChatMember
		new  tgmodels.ChatMember
		want bool
	}{
		{
			name: "left to member",
			old:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeLeft, Left: &tgmodels.ChatMemberLeft{User: &user}},
			new:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeMember, Member: &tgmodels.ChatMemberMember{User: &user}},
			want: true,
		},
		{
			name: "kicked to member",
			old:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeBanned, Banned: &tgmodels.ChatMemberBanned{User: &user}},
			new:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeMember, Member: &tgmodels.ChatMemberMember{User: &user}},
			want: true,
		},
		{
			name: "member to restricted mute",
			old:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeMember, Member: &tgmodels.ChatMemberMember{User: &user}},
			new: tgmodels.ChatMember{
				Type: tgmodels.ChatMemberTypeRestricted,
				Restricted: &tgmodels.ChatMemberRestricted{User: &user, IsMember: true},
			},
			want: false,
		},
		{
			name: "restricted to member unmute",
			old: tgmodels.ChatMember{
				Type: tgmodels.ChatMemberTypeRestricted,
				Restricted: &tgmodels.ChatMemberRestricted{User: &user, IsMember: true},
			},
			new:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeMember, Member: &tgmodels.ChatMemberMember{User: &user}},
			want: false,
		},
		{
			name: "left to administrator",
			old:  tgmodels.ChatMember{Type: tgmodels.ChatMemberTypeLeft, Left: &tgmodels.ChatMemberLeft{User: &user}},
			new: tgmodels.ChatMember{
				Type: tgmodels.ChatMemberTypeAdministrator,
				Administrator: &tgmodels.ChatMemberAdministrator{User: user},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isJoinTransition(tt.old, tt.new))
		})
	}
}

func TestMemberUser(t *testing.T) {
	user := tgmodels.User{ID: 42, FirstName: "x"}
	cm := tgmodels.ChatMember{
		Type:   tgmodels.ChatMemberTypeMember,
		Member: &tgmodels.ChatMemberMember{User: &user},
	}
	got, ok := memberUser(cm)
	assert.True(t, ok)
	assert.Equal(t, int64(42), got.ID)
}
