-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION notify_new_award()
RETURNS trigger 
LANGUAGE plpgsql
AS $$
DECLARE
    payload JSON;
BEGIN
    -- Create a JSON payload containing both award_id and user_id
    payload := json_build_object('award_id', NEW.award_id, 'user_id', NEW.user_id);

    -- Notify the new_award channel, passing the JSON payload as a text
    PERFORM pg_notify('new_award', payload::text);
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- Create the trigger that fires the function after an insert on the awards table
-- +goose StatementBegin
CREATE TRIGGER new_award_trigger
AFTER INSERT ON user_awards
FOR EACH ROW
EXECUTE FUNCTION notify_new_award();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Drop the trigger first
DROP TRIGGER IF EXISTS new_award_trigger ON awards;

-- Drop the trigger function
DROP FUNCTION IF EXISTS notify_new_award();
-- +goose StatementEnd
