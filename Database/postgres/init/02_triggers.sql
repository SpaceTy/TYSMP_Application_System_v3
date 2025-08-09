-- Common updated_at trigger
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

CREATE TRIGGER applications_set_updated_at
BEFORE UPDATE ON applications
FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Notify services when important changes occur
CREATE OR REPLACE FUNCTION notify_app_event() RETURNS trigger AS $$
DECLARE
  payload json;
BEGIN
  payload := json_build_object(
    'table', TG_TABLE_NAME,
    'action', TG_OP,
    'row_id', COALESCE(NEW.id, OLD.id),
    'user_id', COALESCE(NEW.user_id, OLD.user_id),
    'status', COALESCE(NEW.status, OLD.status),
    'minecraft_name', CASE WHEN TG_TABLE_NAME = 'users' THEN COALESCE(NEW.minecraft_name, OLD.minecraft_name) ELSE NULL END,
    'discord_user_id', CASE WHEN TG_TABLE_NAME = 'users' THEN COALESCE(NEW.discord_user_id, OLD.discord_user_id) ELSE NULL END,
    'at', now()
  );
  PERFORM pg_notify('app_events', payload::text);
  RETURN COALESCE(NEW, OLD);
END; $$ LANGUAGE plpgsql;

-- Emit events when:
--  - users.minecraft_name changes (for whitelist sync)
--  - applications.status changes (for role sync)
CREATE TRIGGER users_notify
AFTER INSERT OR UPDATE OF minecraft_name ON users
FOR EACH ROW EXECUTE PROCEDURE notify_app_event();

CREATE TRIGGER applications_notify
AFTER INSERT OR UPDATE OF status ON applications
FOR EACH ROW EXECUTE PROCEDURE notify_app_event();

-- Generic audit trigger capturing before/after
CREATE OR REPLACE FUNCTION audit_row() RETURNS trigger AS $$
BEGIN
  IF (TG_OP = 'INSERT') THEN
    INSERT INTO audit_log(table_name,row_id,action,before_data,after_data,actor)
      VALUES (TG_TABLE_NAME, NEW.id, TG_OP, NULL, to_jsonb(NEW), current_setting('application.actor', true));
    RETURN NEW;
  ELSIF (TG_OP = 'UPDATE') THEN
    INSERT INTO audit_log(table_name,row_id,action,before_data,after_data,actor)
      VALUES (TG_TABLE_NAME, NEW.id, TG_OP, to_jsonb(OLD), to_jsonb(NEW), current_setting('application.actor', true));
    RETURN NEW;
  ELSE
    INSERT INTO audit_log(table_name,row_id,action,before_data,after_data,actor)
      VALUES (TG_TABLE_NAME, OLD.id, TG_OP, to_jsonb(OLD), NULL, current_setting('application.actor', true));
    RETURN OLD;
  END IF;
END; $$ LANGUAGE plpgsql;

CREATE TRIGGER users_audit
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE PROCEDURE audit_row();

CREATE TRIGGER applications_audit
AFTER INSERT OR UPDATE OR DELETE ON applications
FOR EACH ROW EXECUTE PROCEDURE audit_row();

