import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:smart_fin/classes/auth_obj.dart';
import 'package:smart_fin/classes/sign_up_class.dart';

class ApiServiceForSignUp {
  static const String baseUrl = 'https://api.example.com';

  // Отправка пользователя на endpoint
  static Future<bool> createUser(SignUp user) async {
    try {
      final response = await http.post(
        Uri.parse('$baseUrl/auth/signup'),
        // headers: {
        //   'Content-Type': 'application/json',
        //   'Authorization': 'Bearer your_token_here', // если нужно
        // },
        body: json.encode(user.toJson()),
      );

      if (response.statusCode == 201) {
        // print('Пользователь успешно создан');
        return true;
      } else {
        // print('Ошибка: ${response.statusCode} - ${response.body}');
        return false;
      }
    } catch (e) {
      // print('Ошибка при отправке: $e');
      return false;
    }
  }

  // Получение пользователя
  // static Future<User?> getUser(int userId) async {
  //   try {
  //     final response = await http.get(
  //       Uri.parse('$baseUrl/users/$userId'),
  //       headers: {
  //         'Authorization': 'Bearer your_token_here',
  //       },
  //     );

  //     if (response.statusCode == 200) {
  //       final Map<String, dynamic> data = json.decode(response.body);
  //       return User.fromJson(data);
  //     } else {
  //       print('Ошибка получения: ${response.statusCode}');
  //       return null;
  //     }
  //   } catch (e) {
  //     print('Ошибка при получении: $e');
  //     return null;
  //   }
  // }
}
