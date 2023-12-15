#!/bin/bash

# 定义要执行的命令和次数
COMMAND="go run pownew.go"
TIMES=8

echo "即将执行 $COMMAND，总共 $TIMES 次。"

# 使用循环来执行命令
for (( i=1; i<=TIMES; i++ ))
do
   echo "第 $i 次执行..."
      $COMMAND
      done

      echo "命令执行完毕。"

