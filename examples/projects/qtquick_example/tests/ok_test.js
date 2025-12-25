function test() {
    verify(textItem, 'text', 'INIT STATE');
    buttonClick(button); // Button text: 'Click to change state'
    verify(textItem, 'text', 'UPDATED STATE');
}
test();
