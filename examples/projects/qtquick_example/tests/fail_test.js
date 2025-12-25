function test() {
    verify(textItem, 'text', 'this verify will fail');
    buttonClick(button); // Button text: 'Click to change state'
    verify(textItem, 'text', 'UPDATED STATE');
}
test();
