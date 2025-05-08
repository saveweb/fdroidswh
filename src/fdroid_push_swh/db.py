import sqlite3

def init_db()->sqlite3.Connection:
    con = sqlite3.connect('db.sqlite')
    cur = con.cursor()
    cur.execute("""
        CREATE TABLE IF NOT EXISTS apps(
                package TEXT NOT NULL PRIMARY KEY,
                meta_added NOT NULL INTEGER, meta_last_updated NOT NULL INTEGER, meta_source_code NOT NULL TEXT,
                last_save_triggered INTEGER,
                swh_task_id INTEGER, swh_task_status TEXT, swh_snapshot_swhid TEXT, swh_head_revision TEXT, swh_head_revision_date TEXT)""")
    
    cur.execute("CREATE INDEX IF NOT EXISTS apps_meta_added ON apps (meta_added)")
    cur.execute("CREATE INDEX IF NOT EXISTS apps_meta_last_updated ON apps (meta_last_updated)")
    cur.execute("CREATE INDEX IF NOT EXISTS apps_last_save_triggered ON apps (last_save_triggered)")

    return con