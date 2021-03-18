#!/bin/bash

# Update manifests/$MINORVERSION and test/bundle/

REPO_ROOT="$(dirname "$0")/.."

if [ -z "$MINORVERSION" ]; then
    echo "MINORVERSION env. variable must be set!"
    exit 1
fi

# Minor version, to be used as part of the dest. directory
MINORVERSION=${MINORVERSION:-0.1}

# Dummy full version, to be replaced by ART tooling
FULLVERSION=$MINORVERSION.0

OUTDIR="$REPO_ROOT/manifests/$MINORVERSION"
mkdir -p $OUTDIR

# Generate new CSV file
operator-sdk generate bundle --input-dir $REPO_ROOT/deploy --version "$FULLVERSION" --package aws-efs-operator --output-dir $OUTDIR --manifests

# operator-sdk adds manifests/ to the output-dir, remove it
mv $OUTDIR/manifests/* $OUTDIR
rmdir $OUTDIR/manifests

# Update test/bundle too
cp $OUTDIR/* $REPO_ROOT/test/bundle/manifests
