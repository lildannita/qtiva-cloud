function test() {
    verify(label, 'text', 'INIT STATE');
    buttonClick(button); // Button text: 'Click to change state'
    verify(label, 'text', 'UPDATED STATE');
}
test();
