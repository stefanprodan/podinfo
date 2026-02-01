#!/bin/sh
set -e

# This is a simulation of a backup process.

EXIT_CODE=${BACKUP_EXIT:-0}

echo "Starting backup (estimated run time: 60s)"
podcli check http database-replica:3306/readyz
sleep 60
echo "Backup finished"
exit $EXIT_CODE
