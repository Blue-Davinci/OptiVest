-- +goose Up
-- Trigger for awarding the 'first expense added' when a new expense is created
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION award_first_expense()
RETURNS TRIGGER AS $$
BEGIN

    -- Check if the user has already received the "first_expense" award
    IF (SELECT COUNT(*) FROM user_awards ua
        JOIN awards a ON a.id = ua.award_id
        WHERE ua.user_id = NEW.user_id
        AND a.code = 'first_expense') = 0 THEN

        -- Check if this is the user's first expense
        IF (SELECT COUNT(*) FROM expenses WHERE user_id = NEW.user_id) = 1 THEN
            -- Insert the "first_expense" award for the user
            INSERT INTO user_awards (user_id, award_id, created_at)
            SELECT NEW.user_id, a.id, NOW()
            FROM awards a
            WHERE a.code = 'first_expense';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_award_first_expense
AFTER INSERT ON expenses
FOR EACH ROW
EXECUTE FUNCTION award_first_expense();
-- +goose StatementEnd

-- Trigger for awarding the 'first goal created' when a new goal is created
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION award_first_goal()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if the user has already received the "first_goal" award
    IF (SELECT COUNT(*) FROM user_awards ua
        JOIN awards a ON a.id = ua.award_id
        WHERE ua.user_id = NEW.user_id
        AND a.code = 'first_goal') = 0 THEN

        -- Check if this is the user's first goal
        IF (SELECT COUNT(*) FROM goals WHERE user_id = NEW.user_id) = 1 THEN
            -- Insert the "first_goal" award for the user
            INSERT INTO user_awards (user_id, award_id, created_at)
            SELECT NEW.user_id, a.id, NOW()
            FROM awards a
            WHERE a.code = 'first_goal';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_award_first_goal
AFTER INSERT ON goals
FOR EACH ROW
EXECUTE FUNCTION award_first_goal();
-- +goose StatementEnd

-- Trigger for awarding the 'first budget created' when a new budget is created
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION award_first_budget()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if the user has already received the "first_budget" award
    IF (SELECT COUNT(*) FROM user_awards ua
        JOIN awards a ON a.id = ua.award_id
        WHERE ua.user_id = NEW.user_id
        AND a.code = 'first_budget') = 0 THEN

        -- Check if this is the user's first budget
        IF (SELECT COUNT(*) FROM budgets WHERE user_id = NEW.user_id) = 1 THEN
            -- Insert the "first_budget" award for the user
            INSERT INTO user_awards (user_id, award_id, created_at)
            SELECT NEW.user_id, a.id, NOW()
            FROM awards a
            WHERE a.code = 'first_budget';
        END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_award_first_budget
AFTER INSERT ON budgets
FOR EACH ROW
EXECUTE FUNCTION award_first_budget();
-- +goose StatementEnd

-- Trigger for awardin the 'first income created' 
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION award_first_income()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if the user has already received the "first_income" award
    IF (SELECT COUNT(*) FROM user_awards ua
        JOIN awards a ON a.id = ua.award_id
        WHERE ua.user_id = NEW.user_id
        AND a.code = 'first_income') = 0 THEN

        -- Check if this is the user's first income
        IF (SELECT COUNT(*) FROM income WHERE user_id = NEW.user_id) = 0 THEN
            -- Insert the "first_income" award for the user
            INSERT INTO user_awards (user_id, award_id, created_at)
            SELECT NEW.user_id, a.id, NOW()
            FROM awards a
            WHERE a.code = 'first_income';
        END IF;
    END IF;

   
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_award_first_income
AFTER INSERT ON income
FOR EACH ROW
EXECUTE FUNCTION award_first_income();
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_award_first_expense ON expenses;
DROP FUNCTION IF EXISTS award_first_expense();
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_award_first_goal ON goals;
DROP FUNCTION IF EXISTS award_first_goal();
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_award_first_budget ON budgets;
DROP FUNCTION IF EXISTS award_first_budget();
-- +goose StatementEnd

-- +goose StatementBegin
DROP TRIGGER IF EXISTS trigger_award_first_income ON income;
DROP FUNCTION IF EXISTS award_first_income();
-- +goose StatementEnd

