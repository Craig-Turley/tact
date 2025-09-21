CREATE TABLE email_lists (
id INTEGER PRIMARY KEY,
name TEXT NOT NULL
);
CREATE TABLE jobs (
id INTEGER PRIMARY KEY,
name TEXT,
retry_limit INTEGER,
type INTEGER
);
CREATE TABLE scheduling (
id INTEGER PRIMARY KEY,
run_at TEXT NOT NULL,
job_id INTEGER NOT NULL, status INTEGER,
FOREIGN KEY (job_id) REFERENCES "jobs"(id)
);
CREATE TABLE email_job_data (
job_id INTEGER NOT NULL,
list_id INTEGER NOT NULL,
FOREIGN KEY (job_id) REFERENCES "jobs"(id),
FOREIGN KEY (list_id) REFERENCES email_lists(id)
);
CREATE TABLE subscribers
(
id INTEGER PRIMARY KEY,
email TEXT,
first_name TEXT,
last_name TEXT,
list_id INTEGER, is_subscribed BOOLEAN NOT NULL DEFAULT 1 CHECK (is_subscribed IN (0,1)),
FOREIGN KEY (list_id) REFERENCES email_lists(id)
);
