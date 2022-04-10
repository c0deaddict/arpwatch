#! /usr/bin/env nix-shell
#! nix-shell -i python3 -p python3Packages.influxdb

import argparse
from influxdb import InfluxDBClient
from os import getenv


schema = [
    """
    CREATE RETENTION POLICY "8weeks" ON "arpwatch" DURATION 8w REPLICATION 1
    """,
    """
    CREATE RETENTION POLICY "year" ON "arpwatch" DURATION 52w REPLICATION 1
    """,
    """
    CREATE RETENTION POLICY "forever" ON "arpwatch" DURATION 0s REPLICATION 1
    """,
    """
    DROP CONTINUOUS QUERY "host_5m" ON "arpwatch"
    """,
    """
    CREATE CONTINUOUS QUERY "host_5m" ON "arpwatch"
    BEGIN
      SELECT
        count(online) as total,
        sum(online) as online
      INTO "year"."host_5m"
      FROM host
      GROUP BY time(5m), mac
    END
    """,
    """
    DROP CONTINUOUS QUERY "host_1h" ON "arpwatch"
    """,
    """
    CREATE CONTINUOUS QUERY "host_1h" ON "arpwatch"
    BEGIN
      SELECT
        count(online) as total,
        sum(online) as online
      INTO "forever"."host_1h"
      FROM host
      GROUP BY time(1h), mac
    END
    """,
    """
    ALTER RETENTION POLICY "8weeks" ON "arpwatch" DEFAULT
    """,
]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--influx-host",
        help="InfluxDB host, defaults to 'localhost' (env INFLUXDB_HOST)",
        default=getenv("INFLUX_HOST", "localhost"),
    )
    parser.add_argument(
        "--influx-port",
        help="InfluxDB port, defaults to 8086 (env INFLUX_PORT)",
        type=int,
        default=int(getenv("INFLUX_PORT", "8086")),
    )
    parser.add_argument(
        "--influx-db",
        help="InfluxDB database (env INFLUX_DB)",
        default=getenv("INFLUX_DB"),
    )
    args = parser.parse_args()

    influx = InfluxDBClient(host=args.influx_host, port=args.influx_port)
    influx.switch_database(args.influx_db)

    for query in schema:
        influx.query(query)


if __name__ == "__main__":
    main()
