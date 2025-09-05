# Cross-compile settings
CROSS_COMPILE := x86_64-unknown-linux-gnu-
CC := $(CROSS_COMPILE)gcc
CFLAGS := -O2

# Pick first C file in directory and use its basename as target
SRC := $(wildcard *.c)
TARGET := $(basename $(notdir $(firstword $(SRC))))

# Build directories
BUILD_DIR := build

.PHONY: all clean

all: $(TARGET)

# Build Linux binary
$(TARGET): $(SRC)
	mkdir -p $(BUILD_DIR)
	$(CC) $(CFLAGS) $(SRC) -o $(BUILD_DIR)/$(TARGET)

clean:
	rm -rf $(BUILD_DIR)


# brew tap SergioBenitez/osxct
# brew install x86_64-unknown-linux-gnu
