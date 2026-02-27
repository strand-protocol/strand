cmake_minimum_required(VERSION 3.20)
project(strandroute C CXX)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)
set(CMAKE_C_EXTENSIONS OFF)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Options
option(ENABLE_ASAN        "Enable Address Sanitizer"                        OFF)
option(ENABLE_TSAN        "Enable Thread Sanitizer"                         OFF)
option(P4_RUNTIME         "Build P4Runtime / BMv2 Thrift control-plane client" ON)
option(BMV2_THRIFT_ENABLED "Link against BMv2 Thrift libraries (requires BMv2 SDK)" OFF)

# When BMV2_THRIFT_ENABLED is set, the caller must point us at the BMv2 and
# Thrift header / library trees via these cache variables:
set(BMV2_INCLUDE_DIR "" CACHE PATH "Path to BMv2 source root (for Thrift gen-cpp headers)")
set(THRIFT_INCLUDE_DIR "" CACHE PATH "Path to Apache Thrift include directory")
set(THRIFT_LIB_DIR     "" CACHE PATH "Path to Apache Thrift library directory")

# Compiler flags
add_compile_options(
    -Wall -Wextra -Wpedantic -Werror=implicit-function-declaration
    # Security hardening: stack canaries, source fortification, format-string protection
    -fstack-protector-strong
    -D_FORTIFY_SOURCE=2
    -Wformat -Wformat-security -Werror=format-security
)

if(CMAKE_BUILD_TYPE STREQUAL "Release")
    add_compile_options(-O2)
    # RELRO + BIND_NOW: makes GOT read-only after dynamic linking
    add_link_options(-Wl,-z,relro,-z,now)
else()
    add_compile_options(-g -O0)
endif()

if(ENABLE_ASAN)
    add_compile_options(-fsanitize=address,undefined -fno-omit-frame-pointer)
    add_link_options(-fsanitize=address,undefined)
endif()

if(ENABLE_TSAN)
    add_compile_options(-fsanitize=thread)
    add_link_options(-fsanitize=thread)
endif()

# Include directories
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/include)

# ---- StrandRoute Library source files ----

set(STRANDROUTE_SOURCES
    src/sad.c
    src/sad_match.c
    src/routing_table.c
    src/resolver.c
    src/gossip.c
    src/forwarding.c
    src/multipath.c
)

# ---- Optional: P4Runtime control-plane client ----
#
# p4_runtime.c contains a mutex (pthreads) and, when BMV2_THRIFT_ENABLED is
# set, C++ Thrift code.  We therefore compile it as a C++ translation unit
# when Thrift is requested, and as plain C otherwise.
#
# CMake does not support mixing per-file languages in a single target list,
# but we can set the LANGUAGE property on the source file to force the
# C++ compiler, which is backward-compatible for C code.

if(P4_RUNTIME)
    list(APPEND STRANDROUTE_SOURCES src/p4_runtime.c)
    message(STATUS "StrandRoute: P4Runtime client enabled")
    if(BMV2_THRIFT_ENABLED)
        message(STATUS "StrandRoute: BMv2 Thrift mode ENABLED")
        # Force C++ compilation for the Thrift-using translation unit
        set_source_files_properties(src/p4_runtime.c PROPERTIES LANGUAGE CXX)
    else()
        message(STATUS "StrandRoute: P4Runtime running in stub mode (no BMv2 Thrift)")
    endif()
endif()

add_library(strandroute STATIC ${STRANDROUTE_SOURCES})

target_include_directories(strandroute PUBLIC
    ${CMAKE_CURRENT_SOURCE_DIR}/include
)

# Link math library for math functions
target_link_libraries(strandroute PUBLIC m)

# pthreads required by p4_runtime.c mutex (even in stub mode)
if(P4_RUNTIME)
    find_package(Threads REQUIRED)
    target_link_libraries(strandroute PUBLIC Threads::Threads)

    if(BMV2_THRIFT_ENABLED)
        # -DBMV2_THRIFT_ENABLED enables the real Thrift code paths
        target_compile_definitions(strandroute PUBLIC BMV2_THRIFT_ENABLED)

        # BMv2 Thrift-generated headers live in:
        #   <bmv2_src>/targets/simple_switch/thrift/gen-cpp
        if(BMV2_INCLUDE_DIR)
            target_include_directories(strandroute PRIVATE
                "${BMV2_INCLUDE_DIR}/targets/simple_switch/thrift/gen-cpp"
                "${BMV2_INCLUDE_DIR}/src"
            )
        endif()

        if(THRIFT_INCLUDE_DIR)
            target_include_directories(strandroute PRIVATE "${THRIFT_INCLUDE_DIR}")
        endif()

        # Link: thrift core + simple_switch Thrift client
        if(THRIFT_LIB_DIR)
            target_link_directories(strandroute PUBLIC "${THRIFT_LIB_DIR}")
        endif()
        target_link_libraries(strandroute PUBLIC thrift simple_switch_thrift)
    endif()
endif()

# ---- Test Executable ----
enable_testing()

add_executable(strandroute_tests
    tests/test_main.c
    tests/test_sad.c
    tests/test_routing.c
)

target_link_libraries(strandroute_tests PRIVATE strandroute)

add_test(NAME strandroute_tests COMMAND strandroute_tests)
