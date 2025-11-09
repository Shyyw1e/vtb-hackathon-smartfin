import 'package:flutter/material.dart';
import 'package:smart_fin/classes/auth_obj.dart';
import 'package:smart_fin/pages/analysis_page.dart';
import 'package:smart_fin/pages/sign_up_page.dart';
import 'package:smart_fin/services/api_service.dart';

class AuthPage extends StatefulWidget {
  const AuthPage({super.key});

  @override
  State<AuthPage> createState() => _AuthPage();
}

class _AuthPage extends State<AuthPage> {
  final auth_object = User();

  final TextEditingController _passwordController = TextEditingController();
  final TextEditingController _emailController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        children: [
          SizedBox(height: MediaQuery.of(context).size.height * 0.4),
          TextField(
            controller: _passwordController,
            decoration: InputDecoration(
              labelText: 'Пароль',
              hintText: 'Введите ваш пароль',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.person),
            ),
          ),

          SizedBox(height: 20),

          // Поле для email
          TextField(
            controller: _emailController,
            decoration: InputDecoration(
              labelText: 'Email',
              hintText: 'Введите ваш email',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.email),
            ),
            keyboardType: TextInputType.emailAddress,
          ),

          SizedBox(height: 30),

          // Кнопка сохранения
          ElevatedButton(
            onPressed: _saveUser,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.blue,
              foregroundColor: Colors.white,
              padding: EdgeInsets.symmetric(horizontal: 40, vertical: 15),
            ),
            child: Text(
              'Сохранить пользователя',
              style: TextStyle(fontSize: 16),
            ),
          ),
          SizedBox(height: 30),
          Text('ИЛИ'),
          SizedBox(height: 30),
          ElevatedButton(
            onPressed: () {
              Navigator.push(
                context,
                MaterialPageRoute(builder: (context) => SignUpPage()),
              );
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.blue,
              foregroundColor: Colors.white,
              padding: EdgeInsets.symmetric(horizontal: 40, vertical: 15),
            ),
            child: Text('Зарегистрироваться', style: TextStyle(fontSize: 16)),
          ),
        ],
      ),
    );
  }

  void _saveUser() async {
    // Присваиваем значения полям класса
    setState(() {
      auth_object.password = _passwordController.text;
      auth_object.email = _emailController.text;
    });
    final succses = await ApiService.createUser(auth_object);

    if (auth_object.password != '' && auth_object.email != '') {
      // Показываем сообщение об успехе
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Данные пользователя сохранены!'),
          backgroundColor: Colors.green,
          duration: Duration(seconds: 2),
        ),
      );
      await Future.delayed(Duration(seconds: 1));
      Navigator.push(
        context,
        MaterialPageRoute(builder: (context) => AnalysisPage()),
      );
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Некорректный ввод данных'),
          backgroundColor: Colors.red,
          duration: Duration(seconds: 2),
        ),
      );
    }
  }

  @override
  void dispose() {
    // Очищаем контроллеры
    _passwordController.dispose();
    _emailController.dispose();
    super.dispose();
  }
}
