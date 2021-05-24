#!/bin/bash
shopt -s nocasematch
echo -n "Enter test file
Input:"
read account 
tsc examples/$account.ts && node examples/$account.js