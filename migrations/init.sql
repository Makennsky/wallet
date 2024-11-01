-- migrations/init.sql
-- Создаем тип для валют
CREATE TYPE currency AS ENUM ('USD', 'EUR', 'GBP', 'KZT');

-- Создаем таблицу аккаунтов
CREATE TABLE IF NOT EXISTS accounts (
                                        id UUID PRIMARY KEY,
                                        balance DECIMAL(20, 2) NOT NULL DEFAULT 0.00 CHECK (balance >= 0),
    currency currency NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

-- Создаем таблицу транзакций
CREATE TABLE IF NOT EXISTS transactions (
                                            id SERIAL PRIMARY KEY,
                                            from_account UUID NOT NULL REFERENCES accounts(id),
    to_account UUID NOT NULL REFERENCES accounts(id),
    amount DECIMAL(20, 2) NOT NULL CHECK (amount > 0),
    status VARCHAR(50) NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_different_accounts CHECK (from_account != to_account)
    );

-- Создаем индексы
CREATE INDEX idx_transactions_from_account ON transactions(from_account);
CREATE INDEX idx_transactions_to_account ON transactions(to_account);
CREATE INDEX idx_transactions_created_at ON transactions(created_at);

-- Создаем функцию для проверки валют
CREATE OR REPLACE FUNCTION check_currency_match()
RETURNS TRIGGER AS $$
DECLARE
from_currency currency;
    to_currency currency;
BEGIN
    -- Получаем валюты обоих аккаунтов
SELECT currency INTO from_currency FROM accounts WHERE id = NEW.from_account;
SELECT currency INTO to_currency FROM accounts WHERE id = NEW.to_account;

-- Проверяем совпадение валют
IF from_currency != to_currency THEN
        RAISE EXCEPTION 'Currency mismatch: accounts have different currencies';
END IF;

RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Создаем триггер для проверки валют перед вставкой
CREATE TRIGGER check_currency_before_insert
    BEFORE INSERT ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION check_currency_match();

-- Создаем триггер для проверки достаточности средств
CREATE OR REPLACE FUNCTION check_sufficient_balance()
RETURNS TRIGGER AS $$
DECLARE
available_balance DECIMAL(20, 2);
BEGIN
    -- Получаем текущий баланс отправителя
SELECT balance INTO available_balance
FROM accounts
WHERE id = NEW.from_account;

-- Проверяем достаточность средств
IF available_balance < NEW.amount THEN
        RAISE EXCEPTION 'Insufficient funds: available balance is %', available_balance;
END IF;

RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Создаем триггер для проверки баланса перед вставкой
CREATE TRIGGER check_balance_before_insert
    BEFORE INSERT ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION check_sufficient_balance();

-- Создаем функцию для обновления балансов
CREATE OR REPLACE FUNCTION update_balances()
RETURNS TRIGGER AS $$
BEGIN
    -- Уменьшаем баланс отправителя
UPDATE accounts
SET balance = balance - NEW.amount
WHERE id = NEW.from_account;

-- Увеличиваем баланс получателя
UPDATE accounts
SET balance = balance + NEW.amount
WHERE id = NEW.to_account;

RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Создаем триггер для обновления балансов после вставки
CREATE TRIGGER update_balances_after_insert
    AFTER INSERT ON transactions
    FOR EACH ROW
    WHEN (NEW.status = 'completed')
    EXECUTE FUNCTION update_balances();