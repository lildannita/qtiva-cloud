cmake_minimum_required(VERSION 3.16)

project(qtquick_example VERSION 0.1 LANGUAGES CXX)

set(CMAKE_AUTOUIC ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(QT_MIN_VERSION 5.15.2)

find_package(
  Qt5 ${QT_MIN_VERSION}
  COMPONENTS Core
             Quick
  REQUIRED)

set(PROJECT_SOURCES main.cpp qml.qrc)
add_executable(qtquick_example ${PROJECT_SOURCES})
target_link_libraries(qtquick_example PRIVATE Qt5::Core Qt5::Quick)
