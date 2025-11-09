class SignUp {
  String name;
  String password;
  String email;

  SignUp({this.name = '', this.password = '', this.email = ''});

  @override
  String toString() {
    return 'User{name: $name, email: $email}';
  }

  Map<String, dynamic> toJson() {
    return {'name': name, 'password': password, 'email': email};
  }

  // Создание объекта из Map
  factory SignUp.fromJson(Map<String, dynamic> json) {
    return SignUp(
      name: json['name'] ?? '',
      password: json['password'] ?? '',
      email: json['email'] ?? '',
    );
  }
}
