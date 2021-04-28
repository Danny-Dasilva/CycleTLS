#!/bin/bash
shopt -s nocasematch
echo -n "Enter test file
Input:"
read account 
tsc tests/$account.ts && node tests/$account.js