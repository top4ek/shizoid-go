package repositories

// import (
// 	"context"
// 	"database/sql"
// 	"shizoid/internal/models"
// )
//
// type ChatRepository struct {
// 	db *sql.DB
// }
//
// func NewChatRepository(db *sql.DB) *ChatRepository {
// 	return &ChatRepository{db: db}
// }
//
// func (r *ChatRepository) CreateMessage(ctx context.Context, chat *models.Chat) (int64, error) {
// 	var id int64
// 	err := r.db.QueryRowContext(ctx, "INSERT INTO chats (user_id, message) VALUES ($1, $2) RETURNING id", chat.UserID, chat.Message).Scan(&id)
// 	return id, err
// }
//
// func (r *ChatRepository) GetMessagesByUserID(ctx context.Context, userID int64) ([]*models.Chat, error) {
// 	rows, err := r.db.QueryContext(ctx, "SELECT id, user_id, message, created_at FROM chats WHERE user_id = $1", userID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()
//
// 	var chats []*models.Chat
// 	for rows.Next() {
// 		chat := &models.Chat{}
// 		if err := rows.Scan(&chat.ID, &chat.UserID, &chat.Message, &chat.CreatedAt); err != nil {
// 			return nil, err
// 		}
// 		chats = append(chats, chat)
// 	}
// 	return chats, nil
// }
