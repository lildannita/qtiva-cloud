import QtQuick 2.15
import QtQuick.Window 2.15
import QtQuick.Layouts 1.15
import QtQuick.Controls 2.15

Window {
    width: 200
    height: 200
    visible: true
    title: qsTr("Window")

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 9
        spacing: 6

        Text {
            id: textItem

            Layout.fillHeight: true
            Layout.fillWidth: true
            text: qsTr("INIT STATE")
            font.pixelSize: 14
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
        }

        Button {
            Layout.preferredHeight: 50
            Layout.fillWidth: true
            text: qsTr("Click to change state")
            font.pixelSize: 12
            onClicked: {
                textItem.text = "UPDATED STATE";
            }
        }
    }
}
