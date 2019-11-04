#!/bin/sh

tmpfile=`mktemp order.XXXXXXXX`

echo $1 > $tmpfile

echo "{\"posOrderId\":\""`uuidgen`\""}"
