package service

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
	KZT Currency = "KZT"
)

func (c Currency) IsValid() bool {
	switch c {
	case USD, EUR, GBP, KZT:
		return true
	}
	return false
}

type Account struct {
	ID        string    `json:"id,omitempty" validate:"omitempty,uuid4"`
	Balance   float64   `json:"balance,omitempty" validate:"omitempty,min=0"`
	Currency  Currency  `json:"currency" validate:"required,currency"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type Transaction struct {
	ID        int64     `json:"id,omitempty"`
	From      string    `json:"from" validate:"required,uuid4"`
	To        string    `json:"to" validate:"required,uuid4,nefield=From"`
	Amount    float64   `json:"amount" validate:"required,gt=0"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

type Validator struct {
	validator *validator.Validate
}

func NewValidator() *Validator {
	v := &Validator{
		validator: validator.New(),
	}
	v.RegisterCustomValidations()
	return v
}

func (v *Validator) ValidateAccount(acc *Account) []ValidationError {
	var errors []ValidationError

	err := v.validator.Struct(acc)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			valErr := ValidationError{
				Field:   err.Field(),
				Message: getErrorMsg(err),
			}

			// Безопасное добавление значения
			switch value := err.Value().(type) {
			case string:
				valErr.Value = value
			case float64:
				valErr.Value = value
			case int:
				valErr.Value = value
			default:
				valErr.Value = fmt.Sprintf("%v", value)
			}

			errors = append(errors, valErr)
		}
	}

	return errors
}

func (v *Validator) ValidateTransaction(tx *Transaction) []ValidationError {
	var errors []ValidationError

	err := v.validator.Struct(tx)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			valErr := ValidationError{
				Field:   err.Field(),
				Message: getErrorMsg(err),
			}

			// Безопасное добавление значения
			switch value := err.Value().(type) {
			case string:
				valErr.Value = value
			case float64:
				valErr.Value = value
			case int:
				valErr.Value = value
			default:
				valErr.Value = fmt.Sprintf("%v", value)
			}

			errors = append(errors, valErr)
		}
	}

	return errors
}

func (v *Validator) RegisterCustomValidations() {
	v.validator.RegisterValidation("uuid4", validateUUID4)
	v.validator.RegisterValidation("currency", validateCurrency)
}

func validateCurrency(fl validator.FieldLevel) bool {
	if currency, ok := fl.Field().Interface().(Currency); ok {
		return currency.IsValid()
	}
	return false
}

func validateUUID4(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // Пропускаем пустые значения для omitempty
	}
	_, err := uuid.Parse(value)
	return err == nil
}

func getErrorMsg(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("Field %s is required", err.Field())
	case "uuid4":
		return fmt.Sprintf("Field %s must be a valid UUID v4", err.Field())
	case "min":
		return fmt.Sprintf("Field %s must be greater than or equal to %s", err.Field(), err.Param())
	case "gt":
		return fmt.Sprintf("Field %s must be greater than %s", err.Field(), err.Param())
	case "nefield":
		return fmt.Sprintf("Field %s cannot be the same as %s", err.Field(), err.Param())
	case "currency":
		return fmt.Sprintf("Field %s must be one of: USD, EUR, GBP, KZT", err.Field())
	default:
		return fmt.Sprintf("Field %s is invalid", err.Field())
	}
}

func GenerateAccountID() string {
	return uuid.New().String()
}
