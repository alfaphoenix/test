from django.shortcuts import render, redirect
from django.http import HttpResponse, HttpResponseNotFound, Http404

from .models import *
menu = ['O сайте', "Добавить статью", "обратная связь", "Войти"]
# Create your views here.
def index(request):
    posts = Women.objects.all()
    return render(request, 'women/index.html', {'posts': posts, 'menu': menu, 'title': 'Главная страница'})

def about(request):
    return render(request, 'women/about.html', {'menu': menu, 'title':"О сайте"})

def categories(request, catid):
    if request.GET:
        print(request.GET)
    return HttpResponse(f"<h1> Статьи по категориям</h1><p>{catid}</p>")

def archive(request, year):
    if int(year) > 2021 or int(year) < 1990:
        return redirect('home', permanent=True)
    return HttpResponse(f"<h1> архив по годам</h1><p>{year}</p>")

def pageNotFound(request, exception):
    return HttpResponseNotFound('<h1>обломись</h1>')