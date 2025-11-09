class User {
  String password;
  String email;

  User({this.password = '', this.email = ''});

  @override
  String toString() {
    return 'User{name: $password, email: $email}';
  }

  Map<String, dynamic> toJson() {
    return {'password': password, 'email': email};
  }

  // Создание объекта из Map
  factory User.fromJson(Map<String, dynamic> json) {
    return User(password: json['password'], email: json['email']);
  }
}
