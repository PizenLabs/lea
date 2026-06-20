class Greeter:
    def __init__(self, name):
        self.name = name

    def say_hello(self):
        print(f"Hello, {self.name}")

def main():
    greeter = Greeter("World")
    greeter.say_hello()

if __name__ == "__main__":
    main()
