#include "dialog.h"

#include <QApplication>
#include <iostream>

int main(int argc, char *argv[])
{
    std::cout << "QT WIDGETS EXAMPLE APP STARTED" << std::endl;
    QApplication a(argc, argv);
    Dialog w;
    w.show();
    const auto ret = a.exec();
    std::cout << "QT WIDGETS EXAMPLE APP FINISHED" << std::endl;
    return ret;
}
