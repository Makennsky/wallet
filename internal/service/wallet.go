package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"log"
	"net/http"
	"time"
	"wallet/config"

	"github.com/gorilla/mux"
)

type WalletService struct {
	db        *sql.DB
	validator *Validator
	cfg       *config.Config
}

func NewWalletService(db *sql.DB, cfg *config.Config) *WalletService {
	validator := NewValidator()
	validator.RegisterCustomValidations()

	return &WalletService{
		db:        db,
		validator: validator,
		cfg:       cfg,
	}
}

func (ws *WalletService) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var acc Account
	if err := json.NewDecoder(r.Body).Decode(&acc); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Генерируем UUID если не предоставлен
	if acc.ID == "" {
		acc.ID = GenerateAccountID()
	}

	// Устанавливаем баланс по умолчанию если не предоставлен
	if acc.Balance == 0 {
		acc.Balance = 0
	}

	// Валидация входных данных
	if errs := ws.validator.ValidateAccount(&acc); len(errs) > 0 {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := ws.db.BeginTx(ctx, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback()

	// Проверяем существование аккаунта
	var exists bool
	err = tx.QueryRowContext(ctx, `
        SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)
    `, acc.ID).Scan(&exists)

	if err != nil {
		log.Printf("Error checking account existence: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if exists {
		respondWithError(w, http.StatusConflict, "Account already exists")
		return
	}

	// Создаем новый аккаунт
	err = tx.QueryRowContext(ctx, `
        INSERT INTO accounts (id, balance, currency, created_at)
        VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
        RETURNING created_at
    `, acc.ID, acc.Balance, acc.Currency).Scan(&acc.CreatedAt)

	if err != nil {
		log.Printf("Error creating account: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	if err = tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	respondWithJSON(w, http.StatusCreated, acc)
}

func (ws *WalletService) GetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if !isValidUUID(id) {
		respondWithError(w, http.StatusBadRequest, "Invalid account ID format")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var account Account
	err := ws.db.QueryRowContext(ctx, `
        SELECT id, balance, created_at 
        FROM accounts 
        WHERE id = $1
    `, id).Scan(&account.ID, &account.Balance, &account.CreatedAt)

	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "Account not found")
		return
	} else if err != nil {
		log.Printf("Error getting balance: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, account)
}

func (ws *WalletService) Transfer(w http.ResponseWriter, r *http.Request) {
	var tx Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if errs := ws.validator.ValidateTransaction(&tx); len(errs) > 0 {
		respondWithJSON(w, http.StatusBadRequest, map[string]interface{}{
			"errors": errs,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	dbTx, err := ws.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer dbTx.Rollback()

	var fromAcc, toAcc Account
	if tx.From < tx.To {
		err = ws.lockAndGetAccount(ctx, dbTx, tx.From, &fromAcc)
		if err == nil {
			err = ws.lockAndGetAccount(ctx, dbTx, tx.To, &toAcc)
		}
	} else {
		err = ws.lockAndGetAccount(ctx, dbTx, tx.To, &toAcc)
		if err == nil {
			err = ws.lockAndGetAccount(ctx, dbTx, tx.From, &fromAcc)
		}
	}

	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "One or both accounts not found")
		} else {
			log.Printf("Error locking accounts: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	if fromAcc.Balance < tx.Amount {
		respondWithError(w, http.StatusBadRequest, "Insufficient funds")
		return
	}

	_, err = dbTx.ExecContext(ctx, `
        UPDATE accounts 
        SET balance = balance - $1 
        WHERE id = $2
    `, tx.Amount, tx.From)

	if err != nil {
		log.Printf("Error updating source account: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update source account")
		return
	}

	_, err = dbTx.ExecContext(ctx, `
        UPDATE accounts 
        SET balance = balance + $1 
        WHERE id = $2
    `, tx.Amount, tx.To)

	if err != nil {
		log.Printf("Error updating destination account: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to update destination account")
		return
	}

	tx.Status = "completed"
	err = dbTx.QueryRowContext(ctx, `
        INSERT INTO transactions 
        (from_account, to_account, amount, status, created_at)
        VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
        RETURNING id, created_at
    `, tx.From, tx.To, tx.Amount, tx.Status).Scan(&tx.ID, &tx.CreatedAt)

	if err != nil {
		log.Printf("Error recording transaction: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to record transaction")
		return
	}

	if err = dbTx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	respondWithJSON(w, http.StatusOK, tx)
}

func (ws *WalletService) GetTransactionHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if !isValidUUID(id) {
		respondWithError(w, http.StatusBadRequest, "Invalid account ID format")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var exists bool
	err := ws.db.QueryRowContext(ctx, `
        SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1)
    `, id).Scan(&exists)

	if err != nil {
		log.Printf("Error checking account existence: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if !exists {
		respondWithError(w, http.StatusNotFound, "Account not found")
		return
	}

	rows, err := ws.db.QueryContext(ctx, `
        SELECT id, from_account, to_account, amount, status, created_at
        FROM transactions
        WHERE from_account = $1 OR to_account = $1
        ORDER BY created_at DESC
        LIMIT 100
    `, id)

	if err != nil {
		log.Printf("Error getting transaction history: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var tx Transaction
		err := rows.Scan(
			&tx.ID,
			&tx.From,
			&tx.To,
			&tx.Amount,
			&tx.Status,
			&tx.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning transaction: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}
		transactions = append(transactions, tx)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating transactions: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}

	respondWithJSON(w, http.StatusOK, transactions)
}

func (ws *WalletService) lockAndGetAccount(ctx context.Context, tx *sql.Tx, id string, acc *Account) error {
	return tx.QueryRowContext(ctx, `
        SELECT id, balance, created_at 
        FROM accounts 
        WHERE id = $1 
        FOR UPDATE
    `, id).Scan(&acc.ID, &acc.Balance, &acc.CreatedAt)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}
