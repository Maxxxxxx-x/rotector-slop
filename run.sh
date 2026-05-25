#!/bin/bash

rm rotector-slop.sqlite*
goose up
make build && ./tmp/bin/rotector-slop
