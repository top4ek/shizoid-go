package telegram

// AllowedUpdates mirrors Telegram getUpdates/setWebhook default (all types except
// chat_member, message_reaction, message_reaction_count) plus chat_member.
func AllowedUpdates() []string {
	return []string{
		"message",
		"edited_message",
		"channel_post",
		"edited_channel_post",
		"business_message",
		"edited_business_message",
		"deleted_business_messages",
		"inline_query",
		"chosen_inline_result",
		"callback_query",
		"shipping_query",
		"pre_checkout_query",
		"purchased_paid_media",
		"poll",
		"poll_answer",
		"my_chat_member",
		"chat_member",
		"chat_join_request",
		"chat_boost",
		"removed_chat_boost",
	}
}
