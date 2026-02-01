#!/bin/sh
set -e

# This is a simulation of a rollup process.

STEPS=${ROLLUP_STEPS:-6}
echo "Starting rollup with $STEPS steps (estimated run time: $((STEPS * 10))s)"
podcli check http database-replica:3306/readyz
i=1
while [ $i -le $STEPS ]; do
  echo "Running rollup iteration $i of $STEPS"
  sleep 10
  i=$((i + 1))
done
echo "Rollup finished"
