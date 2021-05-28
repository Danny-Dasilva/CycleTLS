#!/bin/bash
while getopts f: flag
do
    case "${flag}" in
        f) filename=${OPTARG};;
    esac
done

tsc examples/$filename.ts && node examples/$filename.js