#include "dialog.h"
#include "./ui_dialog.h"

Dialog::Dialog(QWidget *parent)
    : QDialog(parent)
    , ui(new Ui::Dialog)
{
    ui->setupUi(this);
    connect(ui->pushButton, &QPushButton::clicked, this, [this] {
        ui->label->setText("UPDATED STATE");
    });
}

Dialog::~Dialog()
{
    delete ui;
}
