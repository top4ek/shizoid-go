package repositories

// import (
// 	"context"
// 	"testing"
// 	"time"
//
// 	"example.com/m/v2/internal/models"
// 	"github.com/DATA-DOG/go-sqlmock"
// 	"github.com/stretchr/testify/assert"
// )
//
// func TestChatRepository_CreateMessage(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	assert.NoError(t, err)
// 	defer db.Close()
//
// 	repo := NewChatRepository(db)
// 	chat := &models.Chat{UserID: 1, Message: "hello"}
//
// 	mock.ExpectQuery("INSERT INTO chats \\(user_id, message\\) VALUES \\(\\$1, \\$2\\) RETURNING id").
// 		WithArgs(chat.UserID, chat.Message).
// 		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
//
// 	id, err := repo.CreateMessage(context.Background(), chat)
// 	assert.NoError(t, err)
// 	assert.Equal(t, int64(1), id)
// }
//
// func TestChatRepository_GetMessagesByUserID(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	assert.NoError(t, err)
// 	defer db.Close()
//
// 	repo := NewChatRepository(db)
// 	expectedChats := []*models.Chat{
// 		{ID: 1, UserID: 1, Message: "hello", CreatedAt: time.Now()},
// 		{ID: 2, UserID: 1, Message: "world", CreatedAt: time.Now()},
// 	}
//
// 	rows := sqlmock.NewRows([]string{"id", "user_id", "message", "created_at"}).
// 		AddRow(expectedChats[0].ID, expectedChats[0].UserID, expectedChats[0].Message, expectedChats[0].CreatedAt).
// 		AddRow(expectedChats[1].ID, expectedChats[1].UserID, expectedChats[1].Message, expectedChats[1].CreatedAt)
//
// 	mock.ExpectQuery("SELECT id, user_id, message, created_at FROM chats WHERE user_id = \\$1").
// 		WithArgs(int64(1)).
// 		WillReturnRows(rows)
//
// 	chats, err := repo.GetMessagesByUserID(context.Background(), 1)
// 	assert.NoError(t, err)
// 	assert.Equal(t, expectedChats, chats)
// }
