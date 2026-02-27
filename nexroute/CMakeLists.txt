cmake_minimum_required(VERSION 3.20)
project(nexroute C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)

# Options
option(ENABLE_ASAN  "Enable Address Sanitizer" OFF)
option(ENABLE_TSAN  "Enable Thread Sanitizer"  OFF)

# Compiler flags
add_compile_options(-Wall -Wextra -Wpedantic -Werror=implicit-function-declaration)

if(CMAKE_BUILD_TYPE STREQUAL "Release")
    add_compile_options(-O2)
else()
    add_compile_options(-g -O0)
endif()

if(ENABLE_ASAN)
    add_compile_options(-fsanitize=address -fno-omit-frame-pointer)
    add_link_options(-fsanitize=address)
endif()

if(ENABLE_TSAN)
    add_compile_options(-fsanitize=thread)
    add_link_options(-fsanitize=thread)
endif()

# Include directories
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/include)

# ---- NexRoute Library ----
add_library(nexroute STATIC
    src/sad.c
    src/sad_match.c
    src/routing_table.c
    src/resolver.c
    src/gossip.c
    src/forwarding.c
    src/multipath.c
)

target_include_directories(nexroute PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
)

# Link math library for math functions
target_link_libraries(nexroute PUBLIC m)

# ---- Test Executable ----
enable_testing()

add_executable(nexroute_tests
    tests/test_main.c
    tests/test_sad.c
    tests/test_routing.c
)

target_link_libraries(nexroute_tests PRIVATE nexroute)

add_test(NAME nexroute_tests COMMAND nexroute_tests)
