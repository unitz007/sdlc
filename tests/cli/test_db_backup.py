import os
import tempfile
import shutil
import pytest
from click.testing import CliRunner
from sdlc.cli.db import backup, restore

import sqlalchemy
from sqlalchemy import text

# Import config to monkeypatch defaults
import importlib
app_config = importlib.import_module('app.config')

@pytest.fixture
def temp_dir(tmp_path):
    return str(tmp_path)

def test_backup_and_restore(temp_dir, monkeypatch):
    # Set backup directory to temporary directory and disable compression
    monkeypatch.setattr(app_config, 'DB_BACKUP_DIR', temp_dir)
    monkeypatch.setattr(app_config, 'DB_BACKUP_COMPRESSION', False)

    # Create a temporary SQLite database file
    db_path = os.path.join(temp_dir, 'test.db')
    monkeypatch.setenv('DATABASE_URL', f'sqlite:///{db_path}')

    engine = sqlalchemy.create_engine(f'sqlite:///{db_path}')
    with engine.begin() as conn:
        conn.execute(text('CREATE TABLE foo (id INTEGER PRIMARY KEY, name TEXT)'))
        conn.execute(text("INSERT INTO foo (name) VALUES ('bar')"))

    runner = CliRunner()
    backup_path = os.path.join(temp_dir, 'backup.db')
    # Run backup command
    result = runner.invoke(backup, [backup_path])
    assert result.exit_code == 0, result.output
    assert os.path.isfile(backup_path)

    # Remove original database file to simulate loss
    os.remove(db_path)
    assert not os.path.isfile(db_path)

    # Run restore command
    result = runner.invoke(restore, [backup_path])
    assert result.exit_code == 0, result.output
    assert os.path.isfile(db_path)

    # Verify data restored
    restored_engine = sqlalchemy.create_engine(f'sqlite:///{db_path}')
    with restored_engine.begin() as conn:
        rows = conn.execute(text('SELECT name FROM foo')).fetchall()
        assert rows == [('bar',)]
