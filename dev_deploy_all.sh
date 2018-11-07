#!/usr/bin/env bash

# this just deploys everything under /manifests,
# but tries to space them out a bit to avoid errors.
# in the end, it creates a custom resource to kick
# the operator into action

for FILE in `find ./manifests -name '00-*'`
do
    echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

for FILE in `find ./manifests -name '01-*'`
do
    echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

for FILE in `find ./manifests -name '02-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

for FILE in `find ./manifests -name '03-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

for FILE in `find ./manifests -name '04-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2

for FILE in `find ./manifests -name '05-*'`
do
  echo "creating ${FILE}"
  oc create -f $FILE
done

sleep 2


FILE=examples/cr.yaml
echo "creating ${FILE}"
oc create -f $FILE
