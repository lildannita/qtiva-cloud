function test() {
    verify(label, 'text', 'this verify will fail');
    buttonClick(button); // Button text: 'Click to change state'
    verify(label, 'text', 'UPDATED STATE');
}
test();
