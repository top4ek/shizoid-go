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
// func TestUserRepository_CreateUser(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	assert.NoError(t, err)
// 	defer db.Close()
//
// 	repo := NewUserRepository(db)
// 	user := &models.User{Username: "testuser"}
//
// 	mock.ExpectQuery("INSERT INTO users \\(username\\) VALUES \\(\\$1\\) RETURNING id").
// 		WithArgs(user.Username).
// 		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
//
// 	id, err := repo.CreateUser(context.Background(), user)
// 	assert.NoError(t, err)
// 	assert.Equal(t, int64(1), id)
// }
//
// func TestUserRepository_GetUserByID(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	assert.NoError(t, err)
// 	defer db.Close()
//
// 	repo := NewUserRepository(db)
// 	expectedUser := &models.User{ID: 1, Username: "testuser", CreatedAt: time.Now()}
//
// 	rows := sqlmock.NewRows([]string{"id", "username", "created_at"}).
// 		AddRow(expectedUser.ID, expectedUser.Username, expectedUser.CreatedAt)
//
// 	mock.ExpectQuery("SELECT id, username, created_at FROM users WHERE id = \\$1").
// 		WithArgs(expectedUser.ID).
// 		WillReturnRows(rows)
//
// 	user, err := repo.GetUserByID(context.Background(), expectedUser.ID)
// 	assert.NoError(t, err)
// 	assert.Equal(t, expectedUser, user)
// }
