#!/bin/bash

# Usage function to display help
usage() {
    echo "Usage: $0 [major|minor|patch]"
    exit 1
}

# Check the number of arguments
if [ "$#" -ne 1 ]; then
    usage
fi

# File containing the version
VERSION_FILE="pkg/version/version.go"

# Extract the current version
current_version=$(grep 'var VERSION' $VERSION_FILE | cut -d '"' -f 2)
if [[ -z "$current_version" ]]; then
    echo "Version not found."
    exit 1
fi

# Break down the version into parts
IFS='.' read -ra ADDR <<< "$current_version"
major=${ADDR[0]}
minor=${ADDR[1]}
patch=${ADDR[2]}

# Increment the appropriate version component
case $1 in
    major)
        major=$((major + 1))
        minor=0
        patch=0
        ;;
    minor)
        minor=$((minor + 1))
        patch=0
        ;;
    patch)
        patch=$((patch + 1))
        ;;
    *)
        usage
        ;;
esac

# Assemble the new version
new_version="${major}.${minor}.${patch}"

# Determine OS and adjust sed command accordingly
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/var VERSION = \"$current_version\"/var VERSION = \"$new_version\"/" $VERSION_FILE
else
    sed -i "s/var VERSION = \"$current_version\"/var VERSION = \"$new_version\"/" $VERSION_FILE
fi

echo "Version updated to $new_version"
