#!/bin/bash

command1="$1"
command2="$2"

if [ "$command1" == "" ]; then
  ./core/run.sh

elif [ "$command1" == "push" ]; then

  git add .
  git commit -m "$command2"
  git push origin main

fi