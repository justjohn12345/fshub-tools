
import sqlite3

conn = sqlite3.connect('fshub.db')
c = conn.cursor()

# Create table
c.execute('''
CREATE TABLE flights (
    flightid INTEGER PRIMARY KEY,
    pilotid INTEGER,
    pilotname TEXT,
    landing_rate REAL,
    distance INTEGER,
    "time" INTEGER,
    aircraft_icao TEXT,
    aircraft_name TEXT,
    departure_icao TEXT,
    arrival_icao TEXT,
    fuel_used REAL,
    departure_time DATETIME,
    arrival_time DATETIME
)
''')

c.execute('CREATE INDEX idx_ts ON flights (ts)')

conn.commit()
conn.close()

print("Database initialized.")
