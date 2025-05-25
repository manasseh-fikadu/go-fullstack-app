package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv" // Added for strconv.Itoa
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/mux"
)

// Helper function to create a mock DB and sqlmock instance
func NewMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	return db, mock
}

func TestCreateUserHandler(t *testing.T) {
	db, mock := NewMockDB(t)
	defer db.Close()

	handler := createUser(db) // Get the handler function

	newUser := User{Name: "Test User", Email: "test@example.com"}
	jsonBody, _ := json.Marshal(newUser)
	req, err := http.NewRequest("POST", "/api/go/users", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	// Expectations for QueryRow:
	// We expect an INSERT statement.
	// The arguments are Name, Email, CreatedAt, UpdatedAt.
	// We will check CreatedAt and UpdatedAt are recent.
	// It should return the new user's ID.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users (name, email, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`)).
		WithArgs(newUser.Name, newUser.Email, sqlmock.AnyArg(), sqlmock.AnyArg()). // Using AnyArg for timestamps initially
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))                  // Mock returning ID 1

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Errorf("response body: %s", rr.Body.String())
	}

	var createdUser User
	if err := json.NewDecoder(rr.Body).Decode(&createdUser); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	if createdUser.Id != 1 {
		t.Errorf("expected user ID to be 1, got %d", createdUser.Id)
	}
	if createdUser.Name != newUser.Name {
		t.Errorf("expected user name to be %s, got %s", newUser.Name, createdUser.Name)
	}
	if createdUser.Email != newUser.Email {
		t.Errorf("expected user email to be %s, got %s", newUser.Email, createdUser.Email)
	}

	// Check timestamps - they should be recent (e.g., within a few seconds of now)
	// This is more robust than sqlmock.AnyArg() for the assertion part.
	now := time.Now()
	if createdUser.CreatedAt.IsZero() || now.Sub(createdUser.CreatedAt) > 5*time.Second {
		t.Errorf("expected CreatedAt to be recent, got %v", createdUser.CreatedAt)
	}
	if createdUser.UpdatedAt.IsZero() || now.Sub(createdUser.UpdatedAt) > 5*time.Second {
		t.Errorf("expected UpdatedAt to be recent, got %v", createdUser.UpdatedAt)
	}
	// Check if CreatedAt and UpdatedAt are very close, e.g., within a millisecond
	if createdUser.CreatedAt.Sub(createdUser.UpdatedAt).Abs() > time.Millisecond {
		t.Errorf("expected CreatedAt and UpdatedAt to be very close for new user, got CreatedAt=%v, UpdatedAt=%v, diff=%v", createdUser.CreatedAt, createdUser.UpdatedAt, createdUser.CreatedAt.Sub(createdUser.UpdatedAt).Abs())
	}


	// we make sure that all expectations were met
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestUpdateUserHandler(t *testing.T) {
	db, mock := NewMockDB(t)
	defer db.Close()

	handler := updateUser(db) // Get the handler function

	originalCreatedAt := time.Now().Add(-1 * time.Hour) // An hour ago
	userID := 1
	updateInfo := User{Name: "Updated User", Email: "updated@example.com"} // ID is not in body for PUT, it's in URL
	userIDStr := strconv.Itoa(userID)

	jsonBody, _ := json.Marshal(updateInfo)
	// Need to use mux to extract path variables like {id}
	req, err := http.NewRequest("PUT", "/api/go/users/"+userIDStr, bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req = mux.SetURLVars(req, map[string]string{"id": userIDStr})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	// Expectation for Exec (UPDATE statement)
	// Args: name, email, updated_at, id
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET name = $1, email = $2, updated_at = $3 WHERE id = $4`)).
		WithArgs(updateInfo.Name, updateInfo.Email, sqlmock.AnyArg(), userIDStr). // Use userIDStr (string)
		WillReturnResult(sqlmock.NewResult(1, 1))                                // 1 row affected

	// Expectation for QueryRow (SELECT statement after update)
	// Returns: id, name, email, created_at, updated_at
	// We need to ensure created_at is the original one. Updated_at will be new.
	// The ID passed to QueryRow in the actual code is also the string from mux.Vars.
	rows := sqlmock.NewRows([]string{"id", "name", "email", "created_at", "updated_at"}).
		AddRow(userID, updateInfo.Name, updateInfo.Email, originalCreatedAt, time.Now()) // Mock new updated_at. Note: AddRow provides int for userID, but Scan handles it.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, name, email, created_at, updated_at FROM users WHERE id = $1`)).
		WithArgs(userIDStr). // Use userIDStr (string) here as well.
		WillReturnRows(rows)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
	}

	var updatedUser User
	if err := json.NewDecoder(rr.Body).Decode(&updatedUser); err != nil {
		// If there's an error decoding, print the body for debugging
		if strings.Contains(err.Error(), "EOF") && rr.Body.Len() == 0 {
			t.Fatalf("could not decode response: empty body")
		} else {
			t.Fatalf("could not decode response: %v. Body: %s", err, rr.Body.String())
		}
	}


	if updatedUser.Id != userID {
		t.Errorf("expected user ID to be %d, got %d", userID, updatedUser.Id)
	}
	if updatedUser.Name != updateInfo.Name {
		t.Errorf("expected user name to be %s, got %s", updateInfo.Name, updatedUser.Name)
	}
	if updatedUser.Email != updateInfo.Email {
		t.Errorf("expected user email to be %s, got %s", updateInfo.Email, updatedUser.Email)
	}

	// Check timestamps
	if !updatedUser.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("expected CreatedAt to be unchanged (%v), got %v", originalCreatedAt, updatedUser.CreatedAt)
	}

	now := time.Now()
	if updatedUser.UpdatedAt.IsZero() || now.Sub(updatedUser.UpdatedAt) > 5*time.Second {
		t.Errorf("expected UpdatedAt to be recent, got %v", updatedUser.UpdatedAt)
	}
	if updatedUser.CreatedAt.After(updatedUser.UpdatedAt) {
		t.Errorf("expected CreatedAt (%v) to be before or equal to UpdatedAt (%v)", updatedUser.CreatedAt, updatedUser.UpdatedAt)
	}


	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
